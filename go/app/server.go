package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Server struct {
	// Port is the port number to listen on.
	Port string
	// ImageDirPath is the path to the directory storing images. //画像パス保存
	ImageDirPath string
}

// Run is a method to start the server. //Run→サーバーをスタート。戻り値0なら成功、1なら失敗
// This method returns 0 if the server started successfully, and 1 otherwise.
func (s Server) Run() int {
	// set up logger //ログの設定
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(logger)
	// STEP 4-6: set the log level to DEBUG
	slog.SetLogLoggerLevel(slog.LevelInfo)

	// set up CORS settings
	frontURL, found := os.LookupEnv("FRONT_URL")
	if !found {
		frontURL = "http://localhost:3000"
	}

	// STEP 5-1: set up the database connection

	// set up handlers
	itemRepo := NewItemRepository()
	h := &Handlers{imgDirPath: s.ImageDirPath, itemRepo: itemRepo}

	// set up routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", h.Hello)
	mux.HandleFunc("GET /items", h.GetItem)
	mux.HandleFunc("GET /items/{id}", h.GetAnItem)
	mux.HandleFunc("POST /items", h.AddItem)
	mux.HandleFunc("GET /images/{filename}", h.GetImage)

	// start the server
	slog.Info("http server started on", "port", s.Port)
	err := http.ListenAndServe(":"+s.Port, simpleCORSMiddleware(simpleLoggerMiddleware(mux), frontURL, []string{"GET", "HEAD", "POST", "OPTIONS"}))
	if err != nil {
		slog.Error("failed to start server: ", "error", err)
		return 1
	}

	return 0
}

type Handlers struct {
	// imgDirPath is the path to the directory storing images.
	imgDirPath string
	itemRepo   ItemRepository
}

type HelloResponse struct {
	Message string `json:"message"`
}

// Hello is a handler to return a Hello, world! message for GET / .
func (s *Handlers) Hello(w http.ResponseWriter, r *http.Request) {
	resp := HelloResponse{Message: "Hello, world!"}
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type GetItemsResponse struct {
	Items []*Item `json:"items"`
}

// 4-3
// GetItem is a handler to return a itemdata for GET /items
func (s *Handlers) GetItem(w http.ResponseWriter, r *http.Request) {
	//http.Request に関連付けられたコンテキストオブジェクト(処理落ち、タイムアウトなど)を取得
	ctx := r.Context()

	//itemsはリポジトリに保存されているので、それをリスト化して取得する
	items, err := s.itemRepo.List(ctx)
	if err != nil {
		slog.Error("failed to get items: ", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := GetItemsResponse{Items: items}
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetAnItem is a handler to return an "one" itemdata that have requested item_id for GET /items/{id}
func (s *Handlers) GetAnItem(w http.ResponseWriter, r *http.Request) {
	//http.Request に関連付けられたコンテキストオブジェクト(処理落ち、タイムアウトなど)を取得
	ctx := r.Context()

	//URL内に含まれるidを取得
	pid := r.PathValue("id") //リクエストのURL内に含まれているデータを見るときはPathValueを使う
	if pid == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	//取得したidをintに変換
	id, err := strconv.Atoi(pid) //文字列を整数に変換する関数（strconv）
	if err != nil {
		http.Error(w, "id must be an int", http.StatusBadRequest)
		return
	}

	//idからデータを取得する
	item, err := s.itemRepo.Select(ctx, id)
	if err != nil {
		if errors.Is(err, errItemNotFound) {
			http.Error(w, "item not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to get item: ", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = json.NewEncoder(w).Encode(item)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type AddItemRequest struct {
	Name     string `form:"name"`
	Category string `form:"category"` // STEP 4-2: add a category field //<-Done
	Image    []byte `form:"image"`    // STEP 4-4: add an image field //画像はbyteに変換して保存する
}

type AddItemResponse struct {
	Message string `json:"message"`
}

// parseAddItemRequest parses and validates the request to add an item.
func parseAddItemRequest(r *http.Request) (*AddItemRequest, error) {
	req := &AddItemRequest{
		Name:     r.FormValue("name"),
		Category: r.FormValue("category"), // STEP 4-2: add a category field // <- Done
	}

	// STEP 4-4: add an image field
	uploadedFile, _, err := r.FormFile("image")
	if err != nil {
		return nil, errors.New("image is required")
	}
	defer uploadedFile.Close()

	imageData, err := io.ReadAll(uploadedFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read image file: %w", err)
	}

	req.Image = imageData

	// validate the request
	if req.Name == "" {
		return nil, errors.New("name is required")
	}

	if req.Category == "" { // STEP 4-2: validate the category field //<- Done
		return nil, errors.New("category is required")
	}

	if len(req.Image) == 0 { // STEP 4-4: validate the image field //<-DOne
		return nil, errors.New("Uploaded image is empty")
	}

	return req, nil
}

// AddItem is a handler to add a new item for POST /items .
func (s *Handlers) AddItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	req, err := parseAddItemRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// STEP 4-4: uncomment on adding an implementation to store an image //ファイル名をハッシュ化
	fileName, err := s.storeImage(req.Image)
	if err != nil {
		slog.Error("failed to store image: ", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	item := &Item{
		Name:     req.Name,
		Category: req.Category, // STEP 4-2: add a category field //<-Done
		// STEP 4-4: add an image field
		ImageName: fileName,
	}
	message := fmt.Sprintf("item received: %s", item.Name)
	slog.Info(message)

	err = s.itemRepo.Insert(ctx, item)
	if err != nil {
		slog.Error("failed to store item: ", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := AddItemResponse{Message: message}
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// storeImage stores an image and returns the file path and an error if any.
// this method calculates the hash sum of the image as a file name to avoid the duplication of a same file
// and stores it in the image directory.
func (s *Handlers) storeImage(image []byte) (filePath string, err error) {
	// STEP 4-4: add an implementation to store an image
	// TODO:
	// - calc hash sum
	// - build image file path
	// - check if the image already exists
	// - store image
	// - return the image file path

	//画像をハッシュの文字列にする
	hash := sha256.Sum256(image)
	hashStr := hex.EncodeToString(hash[:])

	//ハッシュ化したものからファイルパスをつくる
	fileName := fmt.Sprintf("%s.jpg", hashStr)
	filePath = filepath.Join(s.imgDirPath, fileName)

	//jsonに保存、2重に保存しないように
	if _, err := os.Stat(filePath); err == nil {
		return filePath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("error checking image existance: %w", err)
	}

	//画像を保存
	if err := StoreImage(filePath, image); err != nil {
		return "", fmt.Errorf("failed to store image: %w", err)
	}

	//ファイル名を返す
	return fileName, nil
}

type GetImageRequest struct {
	FileName string // path value
}

// parseGetImageRequest parses and validates the request to get an image.
func parseGetImageRequest(r *http.Request) (*GetImageRequest, error) {
	req := &GetImageRequest{
		FileName: r.PathValue("filename"), // from path parameter
	}

	// validate the request
	if req.FileName == "" {
		return nil, errors.New("filename is required")
	}

	return req, nil
}

// GetImage is a handler to return an image for GET /images/{filename} .
// If the specified image is not found, it returns the default image.
func (s *Handlers) GetImage(w http.ResponseWriter, r *http.Request) {
	req, err := parseGetImageRequest(r)
	if err != nil {
		slog.Warn("failed to parse get image request: ", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	imgPath, err := s.buildImagePath(req.FileName)
	if err != nil {
		if !errors.Is(err, errImageNotFound) {
			slog.Warn("failed to build image path: ", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// when the image is not found, it returns the default image without an error.
		slog.Debug("image not found", "filename", imgPath)
		imgPath = filepath.Join(s.imgDirPath, "default.jpg")
	}

	slog.Info("returned image", "path", imgPath)
	http.ServeFile(w, r, imgPath)
}

// buildImagePath builds the image path and validates it.
func (s *Handlers) buildImagePath(imageFileName string) (string, error) {
	imgPath := filepath.Join(s.imgDirPath, filepath.Clean(imageFileName))

	// to prevent directory traversal attacks
	rel, err := filepath.Rel(s.imgDirPath, imgPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("invalid image path: %s", imgPath)
	}

	// validate the image suffix
	if !strings.HasSuffix(imgPath, ".jpg") && !strings.HasSuffix(imgPath, ".jpeg") {
		return "", fmt.Errorf("image path does not end with .jpg or .jpeg: %s", imgPath)
	}

	// check if the image exists
	_, err = os.Stat(imgPath)
	if err != nil {
		return imgPath, errImageNotFound
	}

	return imgPath, nil
}
