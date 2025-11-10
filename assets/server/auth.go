package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

func newToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id, key := newSession()

	w.Header().Set("Content-Type", "text/csv")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("%s,%s", id, key)))
}

func newSession() (id, key string) {
	idBuf := make([]byte, 16)
	rand.Read(idBuf)
	id = hex.EncodeToString(idBuf)

	keyBuf := make([]byte, 32)
	rand.Read(keyBuf)
	key = hex.EncodeToString(keyBuf)

	_, ok := session.s[id]
	if ok {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()
	session.s[id] = keyBuf

	go func() {
		session.mu.Lock()
		defer session.mu.Unlock()

		time.Sleep(1 * time.Minute)
		delete(session.s, id)
	}()

	return
}

func verify(id, hash string) (available, ok bool) {
	session.mu.Lock()
	defer func() {
		delete(session.s, id)
		session.mu.Unlock()
	}()

	key, ok := session.s[id]
	if !ok {
		return false, false
	}

	mac := hmac.New(sha512.New, key)
	mac.Write([]byte(id + password))
	expected := mac.Sum(nil)

	actual, err := hex.DecodeString(hash)
	if err != nil {
		return true, false
	}

	return true, hmac.Equal(actual, expected)
}
