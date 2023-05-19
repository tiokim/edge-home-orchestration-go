/*******************************************************************************
 * Copyright 2022 Samsung Electronics All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 *******************************************************************************/

package handler

import (
	mux "github.com/gorilla/mux"
	"net/http"
)

const (
	assetFolder = "web"
)

// Routes registers routes for web UI
func Routes() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/", staticHandler)

	s := r.PathPrefix("/api/v1").Subrouter()
	s.HandleFunc("/memory", memoryHandler).Methods("GET")
	return r
}

func staticHandler(w http.ResponseWriter, r *http.Request) {
	http.FileServer(http.Dir(assetFolder)).ServeHTTP(w, r)
}

func memoryHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("pong"))
}
