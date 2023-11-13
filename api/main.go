package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
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
	w.Header().Set("Access-Control-Allow-Origin", "*") //後でvercelのURLに書き換える
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST,PUT,DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)

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
	case http.MethodDelete:
		type deleteData struct {
			Title string `json:"title"`
			//Categoryid   string `json:"category"`
			//Curriculumid string `json:"curriculum"`
		}

		// リクエストボディを読み取る
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading body: %v", err)
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}

		// リクエストボディの内容をログに記録
		bodyString := string(bodyBytes)
		log.Printf("Request Body: %s", bodyString)

		// リクエストボディを再び利用可能にする
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		decoder := json.NewDecoder(r.Body)
		var deletereadData deleteData
		if err := decoder.Decode(&deletereadData); err != nil {

			log.Printf("fail: json.Decode, %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		delete, err := db.Prepare(
			"DELETE FROM ITEMS WHERE TITLE=?")
		if err != nil {
			log.Printf("delete err")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		delete.Exec(deletereadData.Title)

		// case http.MethodDelete:
		// 	vars := mux.Vars(r)
		// 	title := vars["title"] // パスパラメータからタイトルを取得

		// 	_, err := db.Exec("DELETE FROM ITEMS WHERE TITLE=?", title)
		// 	log.Printf(title + "was Deleted")
		// 	if err != nil {
		// 		log.Printf("fail: db.Exec, %v\n", err)
		// 		w.WriteHeader(http.StatusInternalServerError)
		// 		return
		// 	}

		w.WriteHeader(http.StatusOK)
	case http.MethodPut:
		vars := mux.Vars(r)
		id := vars["id"] // URLからIDを取得
		log.Printf("PUTリクエストが来ました。ID(MADE_DAY)は" + id)

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
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"message": "Item updated successfully"})

		}

		w.WriteHeader(http.StatusNoContent) // 成功時は204 No Contentを返す
	default:
		log.Printf("fail: HTTP Method is %s\n", r.Method)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}

func main() {

	http.HandleFunc("/", handler)
	//r.HandleFunc("/items/{title}", editHandler).Methods("PUT")
	//r.HandleFunc("/items/delete/{title}", handler).Methods("DELETE")

	// ハンドラの登録
	//http.HandleFunc("/", handler)
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
