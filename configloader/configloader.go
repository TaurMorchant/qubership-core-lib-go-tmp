package configloader

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/knadh/koanf/v2"
)

var (
	configInstance = &config{}

	ErrNotInitialized = errors.New("configloader is not initialized")
	inited            bool
	initLock          sync.RWMutex
)

type config struct {
	konf         *koanf.Koanf
	loadSources  []*PropertySource
	writeOpsLock sync.RWMutex
}

func (c *config) setKoanf(koanf *koanf.Koanf) {
	c.writeOpsLock.Lock()
	defer c.writeOpsLock.Unlock()
	c.konf = koanf
}

func (c *config) getKoanf() *koanf.Koanf {
	c.writeOpsLock.RLock()
	defer c.writeOpsLock.RUnlock()
	return c.konf
}

const (
	DefaultHttpBufferHeaderMaxSize = "10240"
	CSHttpBufferHeaderMaxSizeName  = "http.buffer.header.max.size"
)

func Init(propertySources ...*PropertySource) {
	configInstance.setKoanf(koanf.New("."))
	for _, koanfPropertySource := range propertySources {
		limit := 10 * time.Minute
		endTime := time.Now().Add(limit)
		copyOnWriteKoanf := configInstance.getKoanf().Copy()
		for {
			if err := copyOnWriteKoanf.Load(asKoanfProvider(copyOnWriteKoanf, koanfPropertySource.Provider), koanfPropertySource.Parser); err != nil {
				if time.Now().After(endTime) {
					panic(fmt.Errorf("could not init configuration from property source during %v minutes, err: %w", limit, err))
				}
				fmt.Printf("Error during init configuration from property source: %s Retrying...\n", err.Error())
				time.Sleep(5 * time.Second)
			} else {
				configInstance.setKoanf(copyOnWriteKoanf)
				break
			}
		}
	}

	configInstance.writeOpsLock.Lock()
	configInstance.loadSources = propertySources
	configInstance.writeOpsLock.Unlock()
	initLock.Lock()
	inited = true
	initLock.Unlock()
	subscribers.notify(Event{Type: InitedEventT, Data: "TMP DATA"})
}

func IsConfigLoaderInited() bool {
	initLock.RLock()
	defer initLock.RUnlock()
	return inited
}
func InitWithSourcesArray(propertySources []*PropertySource) {
	Init(propertySources...)
}

func GetOrDefault(key string, def interface{}) interface{} {
	koanf := configInstance.getKoanf()
	if koanf == nil {
		return def
	}
	if value := koanf.Get(key); value != nil {
		return value
	}
	return def
}

func GetOrDefaultString(key string, def string) string {
	koanf := configInstance.getKoanf()
	if koanf == nil {
		return def
	}
	if value := koanf.String(key); value != "" {
		return value
	}
	return def
}

// GetKoanf returns koanf instance. All WRITE operations on this instance is forbidden, making a copy is mandatory
// before any WRITE logic
func GetKoanf() *koanf.Koanf {
	configInstance.writeOpsLock.RLock()
	defer configInstance.writeOpsLock.RUnlock()
	return configInstance.konf
}

// Refresh reloads all properties with property sources that is passed to Init function
// Refresh will return ErrNotInitialized if it will be called before Init method
func Refresh() error {
	if configInstance.getKoanf() == nil {
		return fmt.Errorf("cannot proceed refresh operation: %w", ErrNotInitialized)
	}

	refreshing := koanf.New(".")
	configInstance.writeOpsLock.RLock()
	propertySources := configInstance.loadSources
	configInstance.writeOpsLock.RUnlock()
	for _, koanfPropertySource := range propertySources {
		if err := refreshing.Load(asKoanfProvider(refreshing, koanfPropertySource.Provider), koanfPropertySource.Parser); err != nil {
			return err
		}
	}
	configInstance.setKoanf(refreshing)
	subscribers.notify(Event{Type: RefreshedEventT})
	return nil
}
