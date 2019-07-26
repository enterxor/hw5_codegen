package main

import (
	"context"
	"fmt"
	"net/http"
)

func (srv *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	switch r.URL.Path {
	case "/user/profile":
		fmt.Println("trying to use otherApi")
	default:
		fmt.Fprintf(w, "Page not found")
		// 404
	}
}

func (srv *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
	switch r.URL.Path {
	case "/user/profile":
		srv.handlerUserProfeile(w, r)
	default:
		fmt.Fprintf(w, "Page not found")
		// 404
	}
}


func (srv *MyApi) handlerUserProfeile(w http.ResponseWriter, r *http.Request) {
	// заполнение структуры params
	// валидирование параметров
	fmt.Println("")
	var ctx context.Context = r.Context()

	params := ProfileParams{  r.URL.Query()["user"][0]}
	fmt.Printf("Request profile for user'%s'\n",params.Login)

	res, err := srv.Profile(ctx, params)
	if err != nil {
		switch err.(type) {
		case *ApiError:
			err:= err.(*ApiError)
			http.Error(w, "customer internal error", err.HTTPStatus)
		default:
			fmt.Printf("internal error: %+v\n", err)
			http.Error(w, "internal error", 500)
		}
		return
	}
	
	fmt.Fprintf(w,"User.ID=%d\n", res.ID)
	fmt.Fprintln(w,"User.Login="+res.Login)
	fmt.Fprintln(w,"User.FullName="+res.FullName)
	fmt.Fprintf(w,"User.Status=%d\n",res.Status)
	return
	// прочие обработки
}

func (srv *MyApi) handlerUserCreate(w http.ResponseWriter, r *http.Request) {
	// заполнение структуры params
	// валидирование параметров
	fmt.Println("")
	var ctx context.Context = r.Context()
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