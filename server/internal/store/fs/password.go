package fs

import (
	"encoding/json"
	"errors"
	"log"
	"os"

	"strconv"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/context"
	"sourcegraph.com/sourcegraph/rwvfs"
	"src.sourcegraph.com/sourcegraph/server/accesscontrol"
	"src.sourcegraph.com/sourcegraph/store"
)

const (
	passwordDBFilename = "passwords.json"
	passwordBcryptWork = 11
)

// passwordMap maps UIDs (base-10 strings) to bcrypt password hashes.
type passwordMap map[string][]byte

// readPasswordDB reads the password database from disk. If no such
// file exists, an empty map (not a nil map) is returned (and no
// error).
func readPasswordDB(ctx context.Context) (passwordMap, error) {
	f, err := dbVFS(ctx).Open(passwordDBFilename)
	if err != nil {
		if os.IsNotExist(err) {
			return passwordMap{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var pwmap passwordMap
	if err := json.NewDecoder(f).Decode(&pwmap); err != nil {
		return nil, err
	}
	return pwmap, nil
}

// writePasswordDB writes the password database to disk.
func writePasswordDB(ctx context.Context, pwmap passwordMap) (err error) {
	data, err := json.MarshalIndent(pwmap, "", "  ")
	if err != nil {
		return err
	}

	if err := rwvfs.MkdirAll(dbVFS(ctx), "."); err != nil {
		return err
	}
	f, err := dbVFS(ctx).Create(passwordDBFilename)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := f.Close(); err2 != nil {
			if err == nil {
				err = err2
			} else {
				log.Printf("Warning: closing password DB after error (%s) failed: %s.", err, err2)
			}
		}
	}()

	_, err = f.Write(data)
	return err
}

// password is an FS-backed implementation of the Password store.
type password struct{}

var _ store.Password = (*password)(nil)

func (p password) CheckUIDPassword(ctx context.Context, uid int32, password string) error {
	if err := accesscontrol.VerifyUserHasAdminAccess(ctx, "Password.CheckUIDPassword"); err != nil {
		return err
	}
	pwmap, err := readPasswordDB(ctx)
	if err != nil {
		return err
	}

	hash, present := pwmap[strconv.Itoa(int(uid))]
	if !present {
		return &store.UserNotFoundError{UID: int(uid)}
	}

	return bcrypt.CompareHashAndPassword(hash, []byte(password))
}

func (p password) SetPassword(ctx context.Context, uid int32, password string) error {
	if err := accesscontrol.VerifyUserSelfOrAdmin(ctx, "Password.SetPassword", uid); err != nil {
		return err
	}
	if password == "" {
		return errors.New("password must not be empty")
	}

	pwmap, err := readPasswordDB(ctx)
	if err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), passwordBcryptWork)
	if err != nil {
		return err
	}
	pwmap[strconv.Itoa(int(uid))] = hash

	return writePasswordDB(ctx, pwmap)
}
