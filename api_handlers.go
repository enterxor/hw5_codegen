package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (srv *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	switch r.URL.Path {
	case "/user/profile":
		fmt.Println("trying to use otherApi")
	default:
		http.Error(w, "{\"error\": \"unknown method\"}", http.StatusNotFound)
		// 404
	}
}

func (srv *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	switch r.URL.Path {
	case "/user/profile":
		srv.handlerUserProfeile(w, r)
	default:
		resJSON, _ := json.Marshal(map[string]string{"error": "unknown method"})
		http.Error(w, string(resJSON), http.StatusNotFound)
		// 404
	}
}

func (srv *MyApi) handlerUserProfeile(w http.ResponseWriter, r *http.Request) {
	// заполнение структуры params
	// валидирование параметров
	fmt.Println("")
	ctx := r.Context()
	var params *ProfileParams
	if r.Method == http.MethodGet {
		logins, ok := r.URL.Query()["login"] //required
		if !ok || len(logins[0]) < 1 {
			fmt.Printf("missing param user in request profile\n")
			resJSON, _ := json.Marshal(map[string]string{"error": "login must me not empty"})
			http.Error(w, string(resJSON), http.StatusBadRequest)
			return
		}

		params = &ProfileParams{logins[0]}
	} else if r.Method == http.MethodPost {
		r.ParseForm()
		login := r.FormValue("login")
		if len(login) < 1 {
			fmt.Printf("missing param user in request profile\n")
			resJSON, _ := json.Marshal(map[string]string{"error": "login must me not empty"})
			http.Error(w, string(resJSON), http.StatusBadRequest)
			return
		}
		params = &ProfileParams{login}
	}

	fmt.Printf("Request profile for user'%s'\n", params.Login)

	res, err := srv.Profile(ctx, *params)
	if err != nil {
		switch err.(type) {
		case ApiError:
			err := err.(ApiError)
			resJSON, _ := json.Marshal(map[string]string{"error": err.Error()})
			http.Error(w, string(resJSON), err.HTTPStatus)
		default:
			resJSON, _ := json.Marshal(map[string]string{"error": err.Error()})
			http.Error(w, string(resJSON), 500)
		}
		return
	}

	resultJSON, _ := json.Marshal(map[string]interface{}{"response": res, "error": ""})
	fmt.Fprintf(w, string(resultJSON))

	return
	// прочие обработки
}

func (srv *MyApi) handlerUserCreate(w http.ResponseWriter, r *http.Request) {
	// заполнение структуры params
	// валидирование параметров
	fmt.Println("")
	var ctx = r.Context()
	params := CreateParams{}
	res, err := srv.Create(ctx, params)
	if err != nil {
		switch err.(type) {
		default:
			fmt.Printf("internal error: %+v\n", err)
			http.Error(w, "internal error", 500)
		}
		return
	}
	fmt.Println(res.ID)
	return
	// прочие обработки
}
