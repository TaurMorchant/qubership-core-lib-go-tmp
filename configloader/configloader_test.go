package configloader

import (
	"errors"
	"fmt"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
	"os"
	"sync"
	"testing"
	"time"
)

var testParams = YamlPropertySourceParams{ConfigFilePath: "./testdata/application.yaml"}

func TestInit_WithEnvironmentSource(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	os.Setenv("ENV_PROPERTY_SOURCE", "env")
	Init(EnvPropertySource())
	assert.Equal(t, "env", GetKoanf().Get("env.property.source"))
}

func TestInit_WithYamlSource(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	Init(YamlPropertySource(testParams))
	assert.Equal(t, "yaml", GetKoanf().Get("test.var"))
}

func TestInit_TestBaseSources(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	os.Setenv("TEST_VAR", "env")
	InitWithSourcesArray(BasePropertySources(testParams))
	assert.Equal(t, "env", configInstance.konf.Get("test.var"))
	os.Unsetenv("TEST_VAR")
}

func TestGetOrDefault_GetRealValue(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	Init(YamlPropertySource(testParams))
	rawKoanfData := GetOrDefault("slice.var", []string{"test"}).([]interface{})
	var testSlice []string
	for _, sliceElem := range rawKoanfData {
		testSlice = append(testSlice, fmt.Sprint(sliceElem))
	}
	assert.Equal(t, 3, len(testSlice))
	assert.True(t, contains(testSlice, "first"))
	assert.True(t, contains(testSlice, "second"))
	assert.True(t, contains(testSlice, "third"))
	assert.False(t, contains(testSlice, "test"))
}

func TestGetOrDefault_GetDefaultValue(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	Init(YamlPropertySource(testParams))
	testSlice := GetOrDefault("nonexistent.var", []string{"test"}).([]string)
	assert.Equal(t, 1, len(testSlice))
	assert.True(t, contains(testSlice, "test"))
}

func TestGetOrDefaultString_GetRealValue(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	realString := "actualString"
	os.Setenv("ENV_REAL_STRING", realString)
	Init(EnvPropertySource())
	testString := GetOrDefaultString("env.real.string", "defaultString")
	assert.Equal(t, realString, testString)
	assert.NotEqual(t, "defaultString", testString)
}

func TestGetOrDefaultString_GetDefaultValue(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	realString := "actualString"
	Init(EnvPropertySource())
	testString := GetOrDefaultString("non.existing.env", "defaultString")
	assert.NotEqual(t, realString, testString)
	assert.Equal(t, "defaultString", testString)
}

func TestGetKoanf(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	Init(EnvPropertySource())
	assert.Equal(t, configInstance.konf, GetKoanf())
}

func TestCreateCustomPropertySource(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	b := []byte(`{"key": "bytes"}`)
	bytePropertySource := &PropertySource{

		Provider: AsPropertyProvider(rawbytes.Provider(b)),
		Parser:   json.Parser(),
	}

	jsonPropertySource := &PropertySource{
		Provider: AsPropertyProvider(file.Provider("testdata/test.json")),
		Parser:   json.Parser(),
	}

	key := "key"
	// bytePropertySource has higher priority than jsonPropertySource
	Init(jsonPropertySource, bytePropertySource)
	assert.Equal(t, "bytes", GetKoanf().Get(key))
	assert.NotEqual(t, "json", GetKoanf().Get(key))

	// jsonPropertySource has higher priority than  bytePropertySource
	Init(bytePropertySource, jsonPropertySource)
	assert.NotEqual(t, "bytes", GetKoanf().Get(key))
	assert.Equal(t, "json", GetKoanf().Get(key))
}

func TestAllPropertiesAreStoredInFlatenMap(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	os.Setenv("UNFLATTEN_ENV", "unflatten_env")
	defer os.Unsetenv("UNFLATTEN_ENV")
	InitWithSourcesArray(BasePropertySources(testParams))
	mapWithProperties := GetKoanf().Raw()
	assert.Equal(t, "unflatten_env", mapWithProperties["unflatten.env"])
	assert.Equal(t, "unflatten_yaml", mapWithProperties["unflatten.yaml"])
	assert.Equal(t, "flatten_yaml", mapWithProperties["flatten.yaml"])
	assert.Equal(t, "unflatten_yaml_deep_level", mapWithProperties["deep.yaml.level"])
}

func TestCanSetPathToApplicationYamlWithEnvVariable(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	os.Setenv(propertyFilePath, "./testdata/")
	defer os.Unsetenv(propertyFilePath)

	InitWithSourcesArray(BasePropertySources())
	assert.Equal(t, "yaml", GetKoanf().Get("test.var"))
}

func Test_TestEnvOverlapping(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	os.Setenv("DBAAS_AGENT_PORT", "8080")
	os.Setenv("DBAAS_AGENT_PROTO", "tcp")
	defer func() {
		os.Unsetenv("DBAAS_AGENT_PORT")
		os.Unsetenv("DBAAS_AGENT_PROTO")
	}()

	InitWithSourcesArray(BasePropertySources(testParams))
	yamlVal := GetOrDefaultString("dbaas.agent", "-")
	envVal := GetOrDefaultString("dbaas.agent.port", "-")
	// expecting that env vars won't overlap yaml var
	assert.Equal(t, "http://dbaas-agent:8080", yamlVal)
	assert.Equal(t, "8080", envVal)
}

func TestCorrectInitializationWithSeveralPropertySources(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	os.Setenv("ENV_VARIABLE", "env")
	defer os.Unsetenv("ENV_VARIABLE")
	jsonPropertySource := &PropertySource{
		Provider: AsPropertyProvider(file.Provider("testdata/test.json")),
		Parser:   json.Parser(),
	}
	Init(jsonPropertySource, YamlPropertySource(testParams), EnvPropertySource())
	// check that we have properties from all sources
	assert.Equal(t, "json", GetKoanf().String("key"))
	assert.Equal(t, "env", GetKoanf().String("env.variable"))
	assert.Equal(t, "yaml", GetKoanf().String("test.var"))
}

func TestRefreshReturnsErrWhenCalledBeforeInit(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	configInstance.konf = nil
	err := Refresh()
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, ErrNotInitialized))
	assert.Equal(t, ErrNotInitialized, errors.Unwrap(err))
}

func TestRefreshAddsNewPropertiesBasedOnEnvVariables(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	envVarKey := "TEST_REFRESH_ADDS_NEW_PROPERTIES"
	envVarKeyPointDelimeted := "test.refresh.adds.new.properties"
	envVarValue := "variable-value"

	InitWithSourcesArray([]*PropertySource{EnvPropertySource()})
	gotValue := GetOrDefault(envVarKeyPointDelimeted, nil)
	assert.Equal(t, nil, gotValue)

	err := os.Setenv(envVarKey, envVarValue)
	assert.Nil(t, err)
	defer os.Unsetenv(envVarKey)

	err = Refresh()
	assert.Nil(t, err)

	gotValue = GetOrDefault(envVarKeyPointDelimeted, nil)
	assert.Equal(t, envVarValue, gotValue)

	gotValue = GetOrDefaultString(envVarKeyPointDelimeted, "")
	assert.Equal(t, envVarValue, gotValue)
}

func TestRefreshRewritesPropertiesOnEnvVariables(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	envVarKey := "TEST_REFRESH_REWRITES_PROPERTIES"
	envVarKeyPointDelimeted := "test.refresh.rewrites.properties"
	envVarValue := "rewrites-old-value"

	err := os.Setenv(envVarKey, envVarValue)
	assert.Nil(t, err)
	defer os.Unsetenv(envVarKey)

	InitWithSourcesArray([]*PropertySource{EnvPropertySource()})
	gotValue := GetOrDefault(envVarKeyPointDelimeted, nil)
	assert.Equal(t, envVarValue, gotValue)

	newEnvVarValue := "rewrites-new-value"
	err = os.Setenv(envVarKey, newEnvVarValue)
	assert.Nil(t, err)

	err = Refresh()
	assert.Nil(t, err)

	gotValue = GetOrDefault(envVarKeyPointDelimeted, nil)
	assert.Equal(t, newEnvVarValue, gotValue)
}

func TestRefreshDoNotAffectReadOperations(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	InitWithSourcesArray([]*PropertySource{EnvPropertySource()})
	allKeys := configInstance.konf.Keys()

	iterationOverChan := make(chan struct{})

	go func() { // Refresh until stop signal sent
		for {
			select {
			case <-iterationOverChan:
				return
			default:
				Refresh()
			}
		}
	}()

	for i := 0; i < 10; i++ {
		for _, k := range allKeys {
			GetOrDefault(k, nil) // Just check if it panics or not (concurrent map read and map write)
		}
	}
	iterationOverChan <- struct{}{}
}

func TestGetKoanfAfterRefreshReturnsDifferentInstance(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	InitWithSourcesArray([]*PropertySource{EnvPropertySource()})
	konfAfterInit := GetKoanf()
	err := Refresh()
	assert.Nil(t, err)
	konfAfterRefresh := GetKoanf()
	assert.NotSamef(t, konfAfterInit, konfAfterRefresh, "GetKoanf can return link to the different instance after Refresh")
}

func TestInitOperationSavesStateBetweenIterations(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	checkAbsencePropertyProvider := mockPropertyProvider{readOperation: func(*koanf.Koanf) (map[string]interface{}, error) {
		assert.Equal(t, "not-init-yet", GetOrDefaultString("test.var", "not-init-yet"))
		return map[string]interface{}{}, nil
	}}
	checkPresencePropertyProvider := mockPropertyProvider{readOperation: func(*koanf.Koanf) (map[string]interface{}, error) {
		assert.Equal(t, "yaml", GetOrDefaultString("test.var", "not-init-yet"))
		return map[string]interface{}{}, nil
	}}

	InitWithSourcesArray([]*PropertySource{
		EnvPropertySource(),
		{Provider: checkAbsencePropertyProvider},
		YamlPropertySource(testParams),
		{Provider: checkPresencePropertyProvider},
	})
}

func TestRefreshOperationSavesStateOnlyAfterAllIterationsOver(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	propertySourceChanged := false
	overridePropertyProvider := mockPropertyProvider{readOperation: func(*koanf.Koanf) (map[string]interface{}, error) {
		if propertySourceChanged {
			return map[string]interface{}{"test.var": "override-happened"}, nil
		}
		return map[string]interface{}{}, nil
	}}
	checkOverridenValuePropertyProvider := mockPropertyProvider{readOperation: func(k *koanf.Koanf) (map[string]interface{}, error) {
		if propertySourceChanged {
			// Overriden value can be obtained only when Refresh is over, or from argument during refresh operation
			assert.Equal(t, "override-happened", k.String("test.var"))
			assert.Equal(t, "yaml", GetOrDefaultString("test.var", "not-init-yet"))
		}
		return map[string]interface{}{}, nil
	}}

	InitWithSourcesArray([]*PropertySource{
		YamlPropertySource(testParams),
		{Provider: overridePropertyProvider},
		{Provider: checkOverridenValuePropertyProvider},
	})

	propertySourceChanged = true
	err := Refresh()
	assert.Nil(t, err)
}

func TestReadPropertyInsideFirstStageOfInitNotFails(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	propertyProviderThatDependsOnNotYetReadProperty := mockPropertyProvider{readOperation: func(k *koanf.Koanf) (map[string]interface{}, error) {
		assert.Equal(t, "default-value", GetOrDefaultString("not.yet.read.property", "default-value"))
		return map[string]interface{}{}, nil
	}}
	Init(&PropertySource{Provider: propertyProviderThatDependsOnNotYetReadProperty})
}

func TestOldPropertiesStaysOnRefreshFailure(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	err := os.Setenv("INTERESTING_PROPERTY", "old-value")
	assert.Nil(t, err)
	defer os.Unsetenv("INTERESTING_PROPERTY")

	mustFail := false
	failSource := mockPropertyProvider{readOperation: func(k *koanf.Koanf) (map[string]interface{}, error) {
		if mustFail {
			return nil, errors.New("fail in property provider")
		}
		return map[string]interface{}{}, nil
	}}
	sources := []*PropertySource{EnvPropertySource(), {Provider: failSource}}

	InitWithSourcesArray(sources)
	assert.Equal(t, "old-value", GetOrDefaultString("interesting.property", ""))

	err = os.Setenv("INTERESTING_PROPERTY", "new-value")
	assert.Nil(t, err)

	mustFail = true
	assert.NotNil(t, Refresh())
	assert.Equal(t, "old-value", GetOrDefaultString("interesting.property", ""))
}

// Test to check for data race problems by using -race flag
func TestConfigLoaderInitRaceCondition(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	propertyKey := "test.race.property"
	envKey := "TEST_RACE_PROPERTY"
	envValue := "race-sample-value"
	err := os.Setenv(envKey, envValue)
	assert.Nil(t, err)
	defer os.Unsetenv(envKey)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		InitWithSourcesArray([]*PropertySource{EnvPropertySource()})
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		GetOrDefault("test.race.property", nil) // Just check read access from the separate go routine
		wg.Done()
	}()
	assert.Eventually(t, func() bool {
		return GetOrDefault(propertyKey, nil) == envValue
	}, 1*time.Second, 100*time.Millisecond)
	wg.Wait()
}

func TestGetOrDefaultWithoutInit(t *testing.T) {
	initLock.Lock()
	inited = false
	initLock.Unlock()
	configInstance.setKoanf(nil)
	assert.Equal(t, "default-value", GetOrDefault("some.property", "default-value"))
}

func TestGetOrDefaultStringWithoutInit(t *testing.T) {
	initLock.Lock()
	inited = false
	initLock.Unlock()
	configInstance.setKoanf(nil)
	assert.Equal(t, "default-value", GetOrDefaultString("some.property", "default-value"))
}

func TestParallelInitAndRefreshRaceCondition(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		InitWithSourcesArray([]*PropertySource{EnvPropertySource()})
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		Refresh()
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		Refresh()
		wg.Done()
	}()
	wg.Wait()
}

func TestConfigLoaderMultipleInitRaceCondition(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	propertyKey := "test.race.property"
	envKey := "TEST_RACE_PROPERTY"
	envValue := "race-sample-value"
	err := os.Setenv(envKey, envValue)
	assert.Nil(t, err)
	defer os.Unsetenv(envKey)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		InitWithSourcesArray([]*PropertySource{EnvPropertySource()})
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		InitWithSourcesArray([]*PropertySource{EnvPropertySource()})
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		GetOrDefault("test.race.property", nil) // Just check read access from the separate go routine
		wg.Done()
	}()
	assert.Eventually(t, func() bool {
		return GetOrDefault(propertyKey, nil) == envValue
	}, 1*time.Second, 100*time.Millisecond)
	wg.Wait()
}

type mockPropertyProvider struct {
	readOperation func(*koanf.Koanf) (map[string]interface{}, error)
}

func (m mockPropertyProvider) ReadBytes(k *koanf.Koanf) ([]byte, error) {
	panic("implement me")
}

func (m mockPropertyProvider) Read(k *koanf.Koanf) (map[string]interface{}, error) {
	return m.readOperation(k)
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}
