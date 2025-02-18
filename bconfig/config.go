package bconfig

import (
	"context"
	"fmt"
	"sync"

	"github.com/lamber92/go-brick/bconfig/benv"
	"github.com/lamber92/go-brick/bconfig/bstorage"
	"github.com/lamber92/go-brick/bconfig/bstorage/apollo"
	"github.com/lamber92/go-brick/bconfig/bstorage/yaml"
)

var (
	_once sync.Once

	_static  bstorage.Config
	_dynamic bstorage.Config
	_env     benv.Env
)

func Static() bstorage.Config {
	return _static
}

func Dynamic() bstorage.Config {
	return _dynamic
}

func Env() benv.Env {
	return _env
}

type Option struct {
	Type      bstorage.Type
	ConfigDir string
	Filename  string
}

func Init(opt Option) {
	_once.Do(func() {
		var (
			err error
		)
		// load environment info
		_env, err = benv.Get()
		if err != nil {
			panic(err)
		}
		if len(opt.ConfigDir) > 0 {
			yaml.InitRootDir(opt.ConfigDir)
		}
		if len(opt.Filename) > 0 {
			yaml.InitDefaultFilename(opt.Filename)
		} else {
			yaml.InitDefaultFilename(_env.GetName())
		}
		// init config manager from diff way by Type
		switch opt.Type {
		case bstorage.YAML:
			initFromYAML()
		case bstorage.APOLLO:
			if err = initFromApollo(); err != nil {
				panic(err)
			}
		default:
			panic(fmt.Sprintf("Unsupported Config-Type [%d]", opt.Type))
		}
	})
}

func initFromYAML() {
	_static = yaml.NewStatic()
	_dynamic = yaml.NewDynamic()
}

func initFromApollo() error {
	basic, err := yaml.NewStatic().Load(context.Background(), "Apollo", _env.GetName())
	if err != nil {
		return err
	}

	conf := &apollo.Config{}
	if err = basic.Unmarshal(conf); err != nil {
		return err
	}
	_dynamic, err = apollo.New(conf)
	if err != nil {
		return err
	}
	_static = _dynamic

	return nil
}

func Close() {
	switch _static.GetType() {
	case bstorage.YAML:
		_static.Close()
		_dynamic.Close()
	case bstorage.APOLLO:
		_dynamic.Close()
	}
}
