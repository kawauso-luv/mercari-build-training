package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	// STEP 5-1: uncomment this line
	// _ "github.com/mattn/go-sqlite3"
)

var errImageNotFound = errors.New("image not found")

type Item struct {
	ID        int    `db:"id" json:"-"`
	Name      string `db:"name" json:"name"`
	Category  string `db:"category" json:"category"`
	ImageName string `db:"image" json:"image"`
}

// Please run `go generate ./...` to generate the mock implementation
// ItemRepository is an interface to manage items.
//
// interfaceは、関数の引数の定義
//
//go:generate go run go.uber.org/mock/mockgen -source=$GOFILE -package=${GOPACKAGE} -destination=./mock_$GOFILE
type ItemRepository interface {
	Insert(ctx context.Context, item *Item) error
	List(ctx context.Context) ([]*Item, error)
}

// itemRepository is an implementation of ItemRepository
type itemRepository struct {
	// fileName is the path to the JSON file storing items.
	fileName string
}

// NewItemRepository creates a new itemRepository.
func NewItemRepository() ItemRepository {
	return &itemRepository{fileName: "items.json"}
}

// Insert inserts an item into the repository.
func (i *itemRepository) Insert(ctx context.Context, item *Item) error {
	// STEP 4-2: add an implementation to store an item
	file, err := os.OpenFile(i.fileName, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// 既存データを読み込む
	var data struct {
		Items []Item `json:"items"`
	}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil && err.Error() != "EOF" {
		return err
	}

	// 新しい item を追加
	data.Items = append(data.Items, *item)

	// ファイルをクリアして新しい JSON データを書き込む
	file.Seek(0, 0)  // ファイルの先頭に戻る
	file.Truncate(0) // ファイルを空にする

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(data); err != nil {
		return err
	}

	return nil
}

// List get all items
func (i *itemRepository) List(ctx context.Context) ([]*Item, error) {
	//dataに、jsonに保存されている中のitem
	var data struct {
		Items []*Item `json:"items"`
	}

	dataBytes, err := os.ReadFile(i.fileName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	//json.Unmarshal->JSON のバイト列 (dataBytes) を Go の構造体 (data) に変換する関数
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data.Items, nil

}

// StoreImage stores an image and returns an error if any.
// This package doesn't have a related interface for simplicity.
func StoreImage(fileName string, image []byte) error {
	// STEP 4-4: add an implementation to store an image
	if err := os.WriteFile(fileName, image, 0644); err != nil {
		return fmt.Errorf("failed to write image file: %w", err)
	}

	return nil
}
