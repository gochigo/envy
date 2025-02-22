/*
package envy makes working with ENV variables in Go trivial.

* Get ENV variables with default values.
* Set ENV variables safely without affecting the underlying system.
* Temporarily change ENV vars; useful for testing.
* Map all of the key/values in the ENV.
* Loads .env files (by using [godotenv](https://github.com/joho/godotenv/))
* More!
*/
package envy

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"golang.org/x/mod/modfile"
)

var gil = &sync.RWMutex{}
var env = map[string]string{}

func init() {
	Load()
	loadSystemEnv()
}

// Load the ENV variables to the env map
func loadSystemEnv() {
	gil.Lock()
	defer gil.Unlock()

	if os.Getenv("GO_ENV") == "" {
		// if the flag "test.v" is *defined*, we're running as a unit test. Note that we don't care
		// about v.Value (verbose test mode); we just want to know if the test environment has defined
		// it. It's also possible that the flags are not yet fully parsed (i.e. flag.Parsed() == false),
		// so we could not depend on v.Value anyway.
		//
		if v := flag.Lookup("test.v"); v != nil {
			env["GO_ENV"] = "test"
		}
	}

	// set the GOPATH if using >= 1.8 and the GOPATH isn't set
	if os.Getenv("GOPATH") == "" {
		out, err := exec.Command("go", "env", "GOPATH").Output()
		if err == nil {
			gp := strings.TrimSpace(string(out))
			os.Setenv("GOPATH", gp)
		}
	}

	for _, e := range os.Environ() {
		pair := strings.Split(e, "=")
		env[pair[0]] = os.Getenv(pair[0])
	}
}

// Reload the ENV variables. Useful if
// an external ENV manager has been used
func Reload() {
	env = map[string]string{}
	loadSystemEnv()
}

// Load .env files. Files will be loaded in the same order that are received.
// Redefined vars will override previously existing values.
// IE: envy.Load(".env", "test_env/.env") will result in DIR=test_env
// If no arg passed, it will try to load a .env file.
func Load(files ...string) error {

	// If no files received, load the default one
	if len(files) == 0 {
		err := godotenv.Load()
		if err == nil {
			Reload()
		}
		return err
	}

	// We received a list of files
	for _, file := range files {

		// Check if it exists or we can access
		if _, err := os.Stat(file); err != nil {
			// It does not exist or we can not access.
			// Return and stop loading
			return err
		}

		// It exists and we have permission. Load it
		if err := godotenv.Load(file); err != nil {
			return err
		}

		// Reload the env so all new changes are noticed
		Reload()

	}
	return nil
}

// Get a value from the ENV. If it doesn't exist the
// default value will be returned.
func Get(key string, value string) string {
	gil.RLock()
	defer gil.RUnlock()
	if v, ok := env[key]; ok {
		return v
	}
	return value
}

// Get a value from the ENV. If it doesn't exist
// an error will be returned
func MustGet(key string) (string, error) {
	gil.RLock()
	defer gil.RUnlock()
	if v, ok := env[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("could not find ENV var with %s", key)
}

// Set a value into the ENV. This is NOT permanent. It will
// only affect values accessed through envy.
func Set(key string, value string) {
	gil.Lock()
	defer gil.Unlock()
	env[key] = value
}

// MustSet the value into the underlying ENV, as well as envy.
// This may return an error if there is a problem setting the
// underlying ENV value.
func MustSet(key string, value string) error {
	gil.Lock()
	defer gil.Unlock()
	err := os.Setenv(key, value)
	if err != nil {
		return err
	}
	env[key] = value
	return nil
}

// Map all of the keys/values set in envy.
func Map() map[string]string {
	gil.RLock()
	defer gil.RUnlock()
	cp := map[string]string{}
	for k, v := range env {
		cp[k] = v
	}
	return cp
}

// Temp makes a copy of the values and allows operation on
// those values temporarily during the run of the function.
// At the end of the function run the copy is discarded and
// the original values are replaced. This is useful for testing.
//
// WARNING: This function is NOT safe to use from a goroutine or
// from code which may access any Get or Set function from a goroutine
func Temp(f func()) {
	oenv := env
	env = map[string]string{}
	for k, v := range oenv {
		env[k] = v
	}
	defer func() { env = oenv }()
	f()
}

func GoPath() string {
	return Get("GOPATH", "")
}

func GoBin() string {
	return Get("GO_BIN", "go")
}

// CurrentModule will attempt to return the module name from `go.mod`.
// GOPATH isn't supported, no fallback to `CurrentPackage()` anymore.
func CurrentModule() (string, error) {
	moddata, err := ioutil.ReadFile("go.mod")
	if err != nil {
		return "", errors.New("go.mod cannot be read or does not exist")
	}
	packagePath := modfile.ModulePath(moddata)
	if packagePath == "" {
		return "", errors.New("go.mod is malformed")
	}
	return packagePath, nil
}

// Environ returns a copy of strings representing the environment, in the form
// "key=value".
func Environ() []string {
	gil.RLock()
	defer gil.RUnlock()
	var e []string
	for k, v := range env {
		e = append(e, fmt.Sprintf("%s=%s", k, v))
	}
	return e
}
