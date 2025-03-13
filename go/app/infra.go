//test
package app

import (
	"database/sql"
	"context"
	"errors"
	"fmt"
	"os"
	// STEP 5-1: uncomment this line
	_ "github.com/mattn/go-sqlite3"
)

var errImageNotFound = errors.New("image not found")
var errItemNotFound = errors.New("item not found")

type Item struct {
	ID        int    `db:"id" json:"-"`
	Name      string `db:"name" json:"name"`
	Category  string `db:"category_name" json:"category"`
	ImageName string `db:"image_name" json:"image"`
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
	Select(ctx context.Context, id int) (*Item, error)
	Search(ctx context.Context, keyword string)([]*Item, error)
}

// itemRepository is an implementation of ItemRepository
type itemRepository struct {
	// fileName is the path to the JSON file storing items.
	db *sql.DB // SQLite3 のデータベース接続
}

// NewItemRepository creates a new itemRepository.
func NewItemRepository(db *sql.DB) ItemRepository {
	return &itemRepository{db: db}
}

// Insert inserts an item into the repository.
func (i *itemRepository) Insert(ctx context.Context, item *Item) error {
	// STEP 4-2: add an implementation to store an item
	// STEP 5-1: sqlite3に保存するように変更
	// STEP 5-3: Categoryを別テーブルに保存
	var categoryID int

	err := i.db.QueryRowContext(ctx, "SELECT id FROM categories WHERE name = ?", item.Category).Scan(&categoryID) //.Scanで挿入してる
	if err != nil {
		if err == sql.ErrNoRows { // カテゴリーが存在しない場合のみ挿入
			res, insertErr := i.db.ExecContext(ctx, "INSERT INTO categories (name) VALUES (?)", item.Category)
			if insertErr != nil {
				return fmt.Errorf("failed to insert category: %v", insertErr)
			}
			id, lastErr := res.LastInsertId()
			if lastErr != nil {
				return fmt.Errorf("failed to get last insert ID: %v", lastErr)
			}
			categoryID = int(id) // 新しく挿入した ID を categoryID にセット
		} else {
			return fmt.Errorf("failed to query category: %v", err) // DB 接続エラーなどはそのまま返す
		}
	}

	query := `INSERT INTO items (name, category_id, image_name) 
              VALUES (?, ?, ?)`
	
	
	_, err = i.db.ExecContext(ctx, query, item.Name, categoryID, item.ImageName)
    if err != nil {
        return fmt.Errorf("failed to insert item: %v", err)
    }

	return nil
}

// List get all items from db
func (i *itemRepository) List(ctx context.Context) ([]*Item, error) {
	// SQL クエリで items テーブルのデータを取得
    rows, err := i.db.QueryContext(ctx,
		 "SELECT items.id, items.name, categories.name AS category_name, items.image_name FROM items JOIN categories ON items.category_id = categories.id")
    if err != nil {
        return nil, fmt.Errorf("failed to query items: %v", err)
    }
    defer rows.Close()

    // 結果を格納するスライス
    var items []*Item

    // 各行のデータを Item 構造体にマッピング
    for rows.Next() {
        var item Item
        if err := rows.Scan(&item.ID, &item.Name, &item.Category, &item.ImageName); err != nil {
            return nil, fmt.Errorf("failed to scan row: %v", err)
        }
        items = append(items, &item)
    }

    // 結果を返す
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("rows iteration error: %v", err)
    }

    return items, nil

}

// Select select item from id
func (i *itemRepository) Select(ctx context.Context, id int) (*Item, error) {
	//場合分けしてあげる
	//idが0以下はおかしいのでエラー
	if id <= 0 {
		return nil, errItemNotFound
	}

	query := `SELECT items.id, items.name, categories.name AS category_id, items.image_name FROM items WHERE id = ?`
	row := i.db.QueryRowContext(ctx, query, id)

	var item Item
	if err := row.Scan(&item.ID, &item.Name, &item.Category, &item.ImageName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errItemNotFound
		}
		return nil, fmt.Errorf("failed to select item: %v", err)
	}

	return &item, nil

}

// Search 
func (i *itemRepository) Search(ctx context.Context, keyword string) ([]*Item, error) {
	
	query := `SELECT items.id, items.name, categories.name AS category_name, items.image_name 
          FROM items
          JOIN categories ON items.category_id = categories.id
          WHERE items.name LIKE ? OR categories.name LIKE ?`

	// 部分一致検索
	searchTerm := "%" + keyword + "%"

	rows, err := i.db.QueryContext(ctx, query, searchTerm, searchTerm)
	if err != nil {
		return nil, fmt.Errorf("failed to search items: %v", err)
	}
	defer rows.Close()

	var items []*Item
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.Name, &item.Category, &item.ImageName); err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}
		items = append(items, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %v", err)
	}

	return items, nil

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
