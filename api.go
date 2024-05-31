package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

func NewAPIServer(listenAddr string, store Storage) *APIServer {
	return &APIServer{
		listenAddr: listenAddr,
		store:      store,
	}
}

func (s *APIServer) Run() {
	router := mux.NewRouter()
	router.HandleFunc("/login", makeHTTPHandlerFunc(s.handleLogin))
	router.HandleFunc("/account", makeHTTPHandlerFunc(s.handleAccount))
	router.HandleFunc("/account/{id}", authWithJWT(makeHTTPHandlerFunc(s.handleGetAccountByID), s.store))
	router.HandleFunc("/transfer", makeHTTPHandlerFunc(s.handleTransfer))
	log.Println("JSON API Server is running on port: ", s.listenAddr)
	http.ListenAndServe(s.listenAddr, router)
}

func (s *APIServer) handleLogin(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "POST" {
		return fmt.Errorf("METHOD NOT ALLOWED")
	}
	var request LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return err
	}
	r.Body.Close()
	acc, err := s.store.GetAccountByNumber(int(request.Number))
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(acc.EncryptedPassword), []byte(request.Password)); err != nil {
		return err
	}
	tokenString, err := createJWT(acc)
	if err != nil {
		return err
	}

	resp := LoginResponse{Token: tokenString, Number: acc.Number}
	fmt.Println(tokenString)
	return WriteJSON(w, http.StatusOK, resp)
}

func (s *APIServer) handleAccount(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {
		return s.handleGetAccount(w, r)
	}
	if r.Method == "POST" {
		return s.handleCreateAccount(w, r)
	}
	return fmt.Errorf("METHOD NOT ALLOWED")
}

func (s *APIServer) handleGetAccount(w http.ResponseWriter, r *http.Request) error {
	accounts, err := s.store.GetAccounts()
	if err != nil {
		return err
	}
	return WriteJSON(w, http.StatusOK, accounts)
}

func (s *APIServer) handleGetAccountByID(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {
		id, err := getIdFromRequest(r)
		if err != nil {
			return err
		}
		account, err := s.store.GetAccountByID((id))
		if err != nil {
			return err
		}
		return WriteJSON(w, http.StatusOK, account)
	}
	if r.Method == "DELETE" {
		return s.handleDeleteAccount(w, r)
	}
	return fmt.Errorf("METHOD NOT ALLOWED %v", r.Method)
}

func (s *APIServer) handleCreateAccount(w http.ResponseWriter, r *http.Request) error {
	creatAccReq := new(CreateAccountRequest)
	if err := json.NewDecoder(r.Body).Decode(creatAccReq); err != nil {
		return err
	}
	account, err := NewAccount(creatAccReq.FirstName, creatAccReq.LastName, creatAccReq.Password)
	if err != nil {
		return err
	}
	if err := s.store.CreateAccount(account); err != nil {
		return err
	}
	// tokenString, err := createJWT(account)
	// if err != nil {
	// 	return err
	// }
	// fmt.Println(tokenString)
	return WriteJSON(w, http.StatusOK, account)
}

func (s *APIServer) handleDeleteAccount(w http.ResponseWriter, r *http.Request) error {
	id, err := getIdFromRequest(r)
	if err != nil {
		return err
	}
	if err := s.store.DeleteAccount(id); err != nil {
		return err
	}
	return WriteJSON(w, http.StatusOK, map[string]int{"deleted": id})
}

func (s *APIServer) handleTransfer(w http.ResponseWriter, r *http.Request) error {
	transferReq := new(TransferRequest)
	if err := json.NewDecoder(r.Body).Decode(transferReq); err != nil {
		return err
	}
	defer r.Body.Close()
	return WriteJSON(w, http.StatusOK, transferReq)
}

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.WriteHeader(status)
	w.Header().Add("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(v)
}

type APIError struct {
	Error string `json:"error"`
}

type apiFunc func(http.ResponseWriter, *http.Request) error

func makeHTTPHandlerFunc(f apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			WriteJSON(w, http.StatusBadRequest, APIError{Error: err.Error()})
		}
	}
}

type APIServer struct {
	listenAddr string
	store      Storage
}

func getIdFromRequest(r *http.Request) (int, error) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, fmt.Errorf("INVALID ID GIVEN %s", idStr)
	}
	return id, nil
}

// Decorator function to act as a middleware niceeeeeeeee
func authWithJWT(f http.HandlerFunc, s Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Authing with JWT")
		tokenString := r.Header.Get("x-jwt-token")
		token, err := validateJWT(tokenString)
		if err != nil {
			WriteJSON(w, http.StatusForbidden, APIError{Error: "bad permission"})
			log.Println("Cant do validation")
			return
		}
		if !token.Valid {
			WriteJSON(w, http.StatusForbidden, APIError{Error: "bad permission"})
			log.Println("Token not valid")
			return
		}
		userID, err := getIdFromRequest(r)
		if err != nil {
			WriteJSON(w, http.StatusForbidden, APIError{Error: "bad permission"})
			log.Println("Cant get id from request")
			return
		}
		account, err := s.GetAccountByID(userID)
		if err != nil {
			WriteJSON(w, http.StatusForbidden, APIError{Error: "bad permission"})
			log.Println("Cant get account from db")
			return
		}
		claims := token.Claims.(jwt.MapClaims)
		if float64(account.Number) != claims["AccountNumber"] {
			log.Println(reflect.TypeOf(claims["AccountNumber"]))
			log.Printf("Cant match %v %v\n", account.Number, claims["AccountNumber"])
			WriteJSON(w, http.StatusForbidden, APIError{Error: "bad permission"})
			return
		}
		fmt.Println(claims)
		f(w, r)
	}
}

func validateJWT(tokenString string) (*jwt.Token, error) {
	secret := os.Getenv("JWT_SECRET")
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("UNEXPECTED SIGNING TOKEN: %v", token.Header["alg"])
		}

		// hmacSampleSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
		return []byte(secret), nil
	})
}

func createJWT(account *Account) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	claims := &jwt.MapClaims{
		"ExpiresAt":     jwt.NewNumericDate(time.Unix(1516239022, 0)),
		"AccountNumber": account.Number,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
