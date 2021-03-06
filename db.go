package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
)

type store interface {
	putAlert(a alertData) error
	getAlert(id string) []byte
	deleteAlert(id string) error
	backup(w http.ResponseWriter)
	getAlertsByPrefix(prefix string) ([]byte, int, error)
}

type boltStore struct {
	db *bolt.DB
}

func newBoltStore() (*boltStore, error) {
	db, err := bolt.Open("alertboard.db", 0600, nil)
	if err != nil {
		return nil, err
	}
	b := &boltStore{db}

	//create the bucket
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("alerts"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})

	return b, err
}

func (b *boltStore) putAlert(a alertData) error {
	if a.Time.IsZero() {
		a.Time = time.Now()
	}

	if a.Status == "" {
		a.Status = "Open"
	}

	data, err := json.Marshal(a)
	if err != nil {
		return err
	}
	err = b.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("alerts"))
		err := b.Put([]byte(a.ID), data)
		return err
	})
	return err
}

func (b *boltStore) backup(w http.ResponseWriter) {
	err := b.db.View(func(tx *bolt.Tx) error {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="alertboard.db"`)
		w.Header().Set("Content-Length", strconv.Itoa(int(tx.Size())))
		_, err := tx.WriteTo(w)
		return err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (b *boltStore) getAlert(id string) []byte {
	var alert []byte
	b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("alerts"))
		alert = b.Get([]byte(id))
		return nil
	})
	return alert
}

func (b *boltStore) deleteAlert(id string) error {
	err := b.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("alerts"))
		err := b.Delete([]byte(id))
		return err
	})
	return err
}

func (b *boltStore) getAlertsByPrefix(prefix string) ([]byte, int, error) {
	alerts := make([]alertData, 0, 0)
	var alert alertData

	err := b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("alerts"))
		c := b.Cursor()
		p := []byte(prefix)
		for k, v := c.Seek(p); bytes.HasPrefix(k, p) && k != nil; k, v = c.Next() {
			err := json.Unmarshal(v, &alert)
			if err != nil {
				return err
			}
			alerts = append(alerts, alert)
		}
		return nil
	})

	data, _ := json.Marshal(alerts)
	return data, len(alerts), err
}

func (b *boltStore) Close() {
	b.db.Close()
}
