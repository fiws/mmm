package credentials

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/minepkg/minepkg/internals/mojang"
	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

var (
	minepkgAuthServiceFallback = "minepkg"
	minepkgAuthUser            = "minepkg_auth_data"

	mojangAuthService = "minepkg"
	mojangAuthUser    = "mojang_auth_data"
)

// Store stures the minepkg & mojang tokens
type Store struct {
	globalDir string
	// MinepkgServiceName should be different for each api env
	MinepkgServiceName string
	NoKeyRingMode      bool
	MinepkgAuth        *oauth2.Token
	MojangAuth         *mojang.AuthResponse
}

// New creates a new credentials store
func New(globalDir string, serviceName string) (*Store, error) {
	store := &Store{globalDir: globalDir, MinepkgServiceName: serviceName}
	if store.MinepkgServiceName == "" {
		store.MinepkgServiceName = minepkgAuthServiceFallback
	}

	err := store.Find()
	if err != nil {
		return nil, err
	}
	return store, nil
}

// Find tries to find existing credentials
func (s *Store) Find() error {
	// find minepkg credentials
	minepkgAuth, err := keyring.Get(s.MinepkgServiceName, minepkgAuthUser)
	switch err {
	case nil:
		err := json.Unmarshal([]byte(minepkgAuth), &s.MinepkgAuth)
		if err != nil {
			return err
		}
	case keyring.ErrNotFound:
		// wo do nothing here, because mojang credentials might be there
	default:
		// TODO: output should be here in debug mode only
		// fmt.Println("Could not use key store, will default to file store for secrets")
		s.NoKeyRingMode = true
		return s.findFromFiles()
	}

	// find mojang credentials
	mojangAuth, err := keyring.Get(mojangAuthService, mojangAuthUser)
	switch err {
	case nil:
		return json.Unmarshal([]byte(mojangAuth), &s.MojangAuth)
	case keyring.ErrNotFound:
		// no credentials (yet) is fine
		return nil
	default:
		return err
	}
}

// findFromFiles is the same as Find but reads from plain files instead
func (s *Store) findFromFiles() error {
	err := s.readCredentialFile("minepkg-credentials.json", &s.MinepkgAuth)
	if err != nil {
		return err
	}

	return s.readCredentialFile("mojang-credentials.json", &s.MojangAuth)
}

// SetMojangAuth sets `MojangAuth` and persists it to disk
func (s *Store) SetMojangAuth(auth *mojang.AuthResponse) error {
	s.MojangAuth = auth

	authJSONBlob, err := json.Marshal(s.MojangAuth)
	if err != nil {
		return err
	}
	if s.NoKeyRingMode {
		return s.writeCredentialFile("mojang-credentials.json", authJSONBlob)
	}
	return keyring.Set(mojangAuthService, mojangAuthUser, string(authJSONBlob))
}

// SetMinepkgAuth sets `MinepkgAuth` and persists it to disk
func (s *Store) SetMinepkgAuth(auth *oauth2.Token) error {
	s.MinepkgAuth = auth

	authJSONBlob, err := json.Marshal(s.MinepkgAuth)
	if err != nil {
		return err
	}
	if s.NoKeyRingMode {
		return s.writeCredentialFile("minepkg-credentials.json", authJSONBlob)
	}
	return keyring.Set(s.MinepkgServiceName, minepkgAuthUser, string(authJSONBlob))
}

// readCredentialFile is a helper that reads a file from the minepkg config dir
func (s *Store) readCredentialFile(location string, v interface{}) error {
	file := filepath.Join(s.globalDir, location)
	rawCreds, err := ioutil.ReadFile(file)
	switch {
	case err == nil:
		// parse json
		if err := json.Unmarshal(rawCreds, &v); err != nil {
			// ignore error. this usually happens if the disk runs out of space
			// by ignoring it we can let the user login again after sufficient
			// space exists again
			fmt.Printf("WARNING: a credentials file was corrupted. ignoring")
			return nil
		}
		// parsed as expected
		return nil
	case os.IsNotExist(err):
		// no file is fine
		return nil
	default:
		// everything else is not
		return err
	}
}

// writeCredentialFile is a helper that writes a file to the minepkg config dir
func (s *Store) writeCredentialFile(location string, content []byte) error {
	os.MkdirAll(s.globalDir, os.ModePerm)
	credFile := filepath.Join(s.globalDir, location)
	return ioutil.WriteFile(credFile, content, 0700)
}
