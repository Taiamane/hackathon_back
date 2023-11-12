package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oklog/ulid"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

type ItemData struct {
	Category    string `json:"category"`
	Curriculum  string `json:"curriculum"`
	Title       string `json:"title"`
	Link        string `json:"link"`
	Summary     string `json:"summary"`
	Made_day    string `json:"made_day"`
	Updated_day string `json:"updated_day"`
}

// ① GoプログラムからMySQLへ接続
var db *sql.DB

func init() {

	//デプロイ時はここを消す
	err := godotenv.Load(".env")

	if err != nil {
		fmt.Printf("読み込み出来ませんでした: %v", err)
	}

	mysqlUser := os.Getenv("MYSQL_USER")
	mysqlUserPwd := os.Getenv("MYSQL_PASSWORD")
	mysqlHost := os.Getenv("MYSQL_HOST")
	mysqlDatabase := os.Getenv("MYSQL_DATABASE")

	connStr := fmt.Sprintf("%s:%s@%s/%s", mysqlUser, mysqlUserPwd, mysqlHost, mysqlDatabase)

	// ①-2
	_db, err := sql.Open("mysql", connStr)
	if err != nil {
		log.Fatalf("fail: sql.Open, %v\n", err)
	}
	// ①-3
	if err := _db.Ping(); err != nil {
		log.Fatalf("fail: _db.Ping, %v\n", err)
	}
	db = _db
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000") //後でvercelのURLに書き換える
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST,DELETE,PUT, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodOptions:
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST,DELETE,PUT, OPTIONS")
		return
	case http.MethodGet:
		sortKey := r.URL.Query().Get("sort")
		order := "made_day DESC" // デフォルトのソート順（作成日時の降順）

		if sortKey != "" {
			order = fmt.Sprintf("%s %s", sortKey, r.URL.Query().Get("order"))
		}

		curriculum := r.URL.Query().Get("curriculum")
		query := "SELECT CATEGORY, CURRICULUM, TITLE, LINK, SUMMARY, MADE_DAY, UPDATED_DAY FROM ITEMS"

		if curriculum != "" {
			query += " WHERE CURRICULUM = ?"
		}

		query += fmt.Sprintf(" ORDER BY %s", order)

		var rows *sql.Rows
		var err error

		if curriculum != "" {
			rows, err = db.Query(query, curriculum)
		} else {
			rows, err = db.Query(query)
		}

		// rows, err := db.Query(query, curriculum)
		if err != nil {
			log.Printf("fail: db.Query, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		itemsData := make([]ItemData, 0)
		for rows.Next() {
			var u ItemData
			var updatedDay sql.NullString
			if err := rows.Scan(&u.Category, &u.Curriculum, &u.Title, &u.Link, &u.Summary, &u.Made_day, &updatedDay); err != nil {
				log.Printf("fail: rows.Scan, %v\n", err)

				if err := rows.Close(); err != nil { // 500を返して終了するが、その前にrowsのClose処理が必要
					log.Printf("fail: rows.Close(), %v\n", err)
				}
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			u.Updated_day = updatedDay.String
			itemsData = append(itemsData, u)
		}
		bytes, err := json.Marshal(itemsData)
		if err != nil {
			log.Printf("fail: json.Marshal, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(bytes)

	case http.MethodPost:
		time := time.Now()
		Id := ulid.MustNew(ulid.Timestamp(time), nil)

		var requestData struct {
			Category    string `json:"category"`
			Curriculum  string `json:"curriculum"`
			Title       string `json:"title"`
			Link        string `json:"link"`
			Summary     string `json:"summary"`
			Made_day    string `json:"made_day"`
			Updated_day string `json:"updated_day"`
		}

		err := json.NewDecoder(r.Body).Decode(&requestData)
		if err != nil {
			log.Println("リクエスト失敗")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if requestData.Category == "" {
			log.Println("fail: category is empty")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		row, err := db.Exec("INSERT INTO ITEMS(CATEGORY,CURRICULUM,TITLE,LINK,SUMMARY,MADE_DAY) VALUES(?,?,?,?,?,?)", requestData.Category, requestData.Curriculum, requestData.Title, requestData.Link, requestData.Summary, requestData.Made_day)
		if err != nil {
			log.Printf("fail: db.Exec, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if row != nil { //post成功時
			w.WriteHeader(http.StatusAccepted)
			allUsers := map[string]string{"id": Id.String()}
			response, err := json.Marshal(allUsers)
			if err != nil {
				log.Printf("fail: json.Marshal, %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(response)
		}
	default:
		log.Printf("fail: HTTP Method is %s\n", r.Method)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}

func detailHandler(w http.ResponseWriter, r *http.Request) {
	// w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000") // 後でVercelのURLに変更
	// w.Header().Set("Access-Control-Allow-Methods", "GET")
	// w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	vars := mux.Vars(r)
	id := vars["id"]

	row := db.QueryRow("SELECT CATEGORY, CURRICULUM, TITLE, LINK, SUMMARY, MADE_DAY, UPDATED_DAY FROM ITEMS WHERE MADE_DAY = ?", id)

	var item ItemData
	err := row.Scan(&item.Category, &item.Curriculum, &item.Title, &item.Link, &item.Summary, &item.Made_day, &item.Updated_day)
	if err != nil {
		log.Printf("fail: db.QueryRow, %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	bytes, err := json.Marshal(item)
	if err != nil {
		log.Printf("fail: json.Marshal, %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(bytes)
}

func editHandler(w http.ResponseWriter, r *http.Request) {
	// w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000") // 後でVercelのURLに変更
	// w.Header().Set("Access-Control-Allow-Methods", "PUT")
	// w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	vars := mux.Vars(r)
	id := vars["id"]

	var requestData ItemData
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		log.Printf("fail: json.NewDecoder, %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	_, err = db.Exec("UPDATE ITEMS SET CATEGORY=?, CURRICULUM=?, TITLE=?, LINK=?, SUMMARY=?, UPDATED_DAY=? WHERE MADE_DAY=?", requestData.Category, requestData.Curriculum, requestData.Title, requestData.Link, requestData.Summary, time.Now(), id)
	if err != nil {
		log.Printf("fail: db.Exec, %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000") // 後でVercelのURLに変更
	w.Header().Set("Access-Control-Allow-Methods", "DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	vars := mux.Vars(r)
	id := vars["id"]

	_, err := db.Exec("DELETE FROM ITEMS WHERE MADE_DAY=?", id)
	if err != nil {
		log.Printf("fail: db.Exec, %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handler_test(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*") // 後でVercelのURLに変更
	w.Header().Set("Access-Control-Allow-Methods", "OPTIONS,GET,POST,DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodOptions:
		w.WriteHeader(http.StatusOK)
		return
	// case http.MethodGet:
	// 	log.Println("hello")
	default:
		log.Printf("fail: HTTP Method is %s\n", r.Method)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}

func main() {

	// ハンドラの登録
	http.HandleFunc("/", handler)
	// http.HandleFunc("/items", handler_test)
	// http.HandleFunc("/items/{id}", detailHandler)
	// http.HandleFunc("/items/{id}", editHandler)
	// http.HandleFunc("/items/{id}", deleteHandler)

	closeDBWithSysCall()

	log.Println("Listening...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func closeDBWithSysCall() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		s := <-sig
		log.Printf("received syscall, %v", s)

		if err := db.Close(); err != nil {
			log.Fatal(err)
		}
		log.Printf("success: db.Close()")
		os.Exit(0)
	}()
}
