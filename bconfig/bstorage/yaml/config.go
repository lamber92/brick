package yaml

import (
	"context"
	"fmt"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/lamber92/go-brick/bconfig/bstorage"
	"github.com/lamber92/go-brick/berror"
	"github.com/lamber92/go-brick/blog/logger"
	"github.com/lamber92/go-brick/btrace"
	"github.com/lamber92/go-brick/internal/bufferpool"
	bsync "github.com/lamber92/go-brick/internal/sync"
	"github.com/spf13/viper"
)

const (
	defaultRoot = "./config"

	configTypeStatic  = "static"
	configTypeDynamic = "dynamic"
)

var (
	_root            = defaultRoot
	_once            sync.Once
	_defaultFilename = "config"
)

// InitRootDir specify the configuration file root directory path.
// the default root path is the config directory under the current directory
func InitRootDir(dir string) {
	_once.Do(func() {
		_root = dir
	})
}

// InitDefaultFilename specify the configuration default filename path.
func InitDefaultFilename(filename string) {
	_defaultFilename = filename
}

// NewStatic new a static config handler(load configuration once).
// throughout the lifetime, the configuration is read only once, and the value is cached.
// calling again will fetch the data in the cache.
func NewStatic() bstorage.Config {
	return newConfig(false)
}

// NewDynamic new a dynamic config handler.
// load real-time configuration values, but allow for slight delays.
func NewDynamic() bstorage.Config {
	return newConfig(true)
}

func newConfig(dynamic bool) *yamlConfig {
	return &yamlConfig{
		config:  sync.Map{},
		lock:    bsync.NewSpinLock(),
		dynamic: dynamic,
	}
}

type yamlConfig struct {
	config    sync.Map
	lock      sync.Locker
	dynamic   bool
	eventHook bstorage.OnChangeFunc
}

func (c *yamlConfig) GetType() bstorage.Type {
	return bstorage.YAML
}

func (c *yamlConfig) Load(ctx context.Context, key string, filenames ...string) (out bstorage.Value, err error) {
	var filename = _defaultFilename
	if len(filenames) > 0 {
		filename = filenames[0]
	}
	defer func() {
		if err == nil {
			btrace.AppendMDIntoCtx(ctx, newMetadata(filename, key, out))
		}
	}()

	// try to get from cache
	cache, ok := c.config.Load(filename)
	if ok {
		out, err = c.handleResult(cache.(*viper.Viper), key)
		return
	}
	// the time gap between Load() and here is very short.
	// ignore the fact that another thread has completed the execution of this method in this gap,
	// resulting in multiple readings of the same file and running multiple watchers.
	c.lock.Lock()
	defer c.lock.Unlock()
	// try again, possibly another thread has already read the configuration and cached it.
	cache, ok = c.config.Load(filename)
	if ok {
		out, err = c.handleResult(cache.(*viper.Viper), key)
		return
	}

	// read config file
	newData, err := c.loadFromFile(c.generateDir(), filename)
	if err != nil {
		return nil, err
	}
	if newData == nil {
		return nil, c.notfoundError(key)
	}

	if c.dynamic {
		// run watcher
		newData.OnConfigChange(func(in fsnotify.Event) {
			if c.eventHook == nil {
				c.onChange(in.String())
			} else {
				c.eventHook(in.String())
			}
		})
		newData.WatchConfig()
	}

	// cache config
	// do not check key is existing or not
	c.config.Store(filename, newData)

	out, err = c.handleResult(newData, key)
	return
}

// RegisterOnChange register callback function for configuration changing notification
// nb. this function is not thread-safe.
func (c *yamlConfig) RegisterOnChange(f bstorage.OnChangeFunc) {
	c.eventHook = f
}

func (c *yamlConfig) Close() {
	return
}

func (c *yamlConfig) onChange(event string) {
	logger.Infra.Infow("[EVENT] config change", logger.NewField().Any("event", event))
}

func (c *yamlConfig) loadFromFile(dir, filename string) (*viper.Viper, error) {
	v := viper.New()
	v.AddConfigPath(dir)
	v.SetConfigName(filename)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		return nil, berror.Convert(err, fmt.Sprintf("Failed to load config: [%s/%s]", dir, filename), nil)
	}
	return v, nil
}

func (c *yamlConfig) generateDir() string {
	buff := bufferpool.Get()
	buff.AppendString(_root)
	// buff.AppendByte(os.PathSeparator)
	buff.AppendByte('/')
	if c.dynamic {
		buff.AppendString(configTypeDynamic)
	} else {
		buff.AppendString(configTypeStatic)
	}
	out := buff.String()
	buff.Free()
	return out
}

func (c *yamlConfig) handleResult(v *viper.Viper, key string) (bstorage.Value, error) {
	sub := v.Sub(key)
	if sub == nil {
		return nil, c.notfoundError(key)
	}
	value := newDefaultValue(sub)
	return value, nil
}

func (c *yamlConfig) notfoundError(key string) error {
	return berror.NewNotFound(nil, fmt.Sprintf("Cannot find key[%s]", key))
}
