package server

import (
	"errors"
	"math"
	"sync"
)

type Store struct {
	lock sync.Mutex
	data map[string]string
	wal  *Wal
}

type KeyValuePair struct {
	key   string
	value string
}

func NewStore() *Store {
	store := &Store{
		lock: sync.Mutex{},
		data: make(map[string]string),
		// wal:  BuildWal("wal.log", &store),
	}

	store.wal = BuildWal("wal.log", &store.data)

	return store
}

func (s *Store) Set(key string, value string) (string, error) {

	if err := validateKey(key); err != nil {
		return "", err
	}

	if err := validateValue(value); err != nil {
		return "", err
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	s.wal.Write(key, value)

	s.data[key] = value

	return value, nil
}

func (s *Store) Get(key string) (string, error) {
	return s.data[key], nil
}

func (s *Store) Delete(key string) (string, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	value := s.data[key]
	delete(s.data, key)

	return value, nil
}

func (s *Store) AsyncList() <-chan KeyValuePair {

	pairs := make(chan KeyValuePair)

	go func() {
		for key := range s.data {
			pairs <- KeyValuePair{
				key:   key,
				value: s.data[key],
			}
		}

		close(pairs)
	}()

	return pairs
}

/////////// Validations

func validateKey(key string) error {
	if key == "" {
		return errors.New("Key must not be blank")
	}

	if len(key) > math.MaxUint16 {
		return errors.New("Key must have fewer than 65536 characters")
	}

	return nil
}

func validateValue(value string) error {
	if len(value) > math.MaxUint32 {
		return errors.New("Value must have fewer than 4,294,967,296 characters. That's a lot of characters. What are you trying to put in here?? That's a 4 GB file! Gtfoh")
	}

	return nil
}
