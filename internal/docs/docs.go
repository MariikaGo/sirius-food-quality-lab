package docs

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func Mount(mux *http.ServeMux, stage int) {
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html lang="ru"><head><meta charset="utf-8"><title>Сириус.Еда API</title><link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui.css"></head><body><div id="swagger-ui"></div><script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script><script>SwaggerUIBundle({url:'/openapi.json',dom_id:'#swagger-ui',persistAuthorization:false})</script></body></html>`)
	})
	mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Spec(stage))
	})
}
func Spec(stage int) map[string]any {
	paths := map[string]any{}
	add := func(method, path, tag string, fields map[string]any, auth bool) {
		op := map[string]any{"tags": []string{tag}, "summary": method + " " + path, "responses": map[string]any{"200": map[string]any{"description": "Успех"}, "400": map[string]any{"description": "Ошибка параметров"}, "401": map[string]any{"description": "Нужен токен"}, "403": map[string]any{"description": "Нет доступа"}}}
		if auth {
			op["security"] = []any{map[string]any{"oauth": []string{}}}
		}
		params := []any{}
		for _, name := range []string{"id"} {
			if contains(path, "{"+name+"}") {
				params = append(params, map[string]any{"in": "path", "name": name, "required": true, "schema": map[string]any{"type": "integer", "minimum": 1}})
			}
		}
		if path == "/users/{id}/topup" {
			params = append(params, map[string]any{"in": "query", "name": "amount", "required": true, "schema": map[string]any{"type": "number", "example": 100}})
		}
		if len(params) > 0 {
			op["parameters"] = params
		}
		if fields != nil {
			op["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "properties": fields}}}}
		}
		entry, ok := paths[path].(map[string]any)
		if !ok {
			entry = map[string]any{}
		}
		entry[method] = op
		paths[path] = entry
	}
	str := func(example string) map[string]any { return map[string]any{"type": "string", "example": example} }
	num := func(example int) map[string]any { return map[string]any{"type": "number", "example": example} }
	fields := map[string]any{"email": str("new@example.com"), "phone": str("+79990000000")}

	fields["password"] = str("StrongPass123!")

	add("post", "/users/register", "users", fields, false)
	add("post", "/users/{id}/topup", "users", nil, true)
	add("get", "/menu/items", "menu", nil, false)
	add("post", "/menu/items", "menu", map[string]any{"name": str("Суп"), "price": num(100), "category_id": num(1)}, true)
	add("patch", "/menu/items/{id}", "menu", map[string]any{"price": num(150), "is_available": map[string]any{"type": "boolean", "example": true}}, true)
	of := map[string]any{"item_id": num(1), "quantity": num(1)}

	add("post", "/orders/create", "orders", of, true)
	add("post", "/delivery/assign", "delivery", map[string]any{"order_id": num(1), "courier_id": num(4)}, true)
	add("patch", "/delivery/{id}/complete", "delivery", nil, true)

	add("get", "/users/me", "users", nil, true)
	add("get", "/orders/kitchen/orders", "orders", nil, true)
	add("get", "/orders/{id}", "orders", nil, true)
	add("post", "/orders/{id}/cancel", "orders", nil, true)
	add("post", "/orders/{id}/paid", "orders", nil, true)
	add("get", "/delivery/orders/{id}", "delivery", nil, true)
	add("post", "/auth/refresh", "auth", map[string]any{"refresh_token": str("вставьте refresh-токен")}, false)
	paths["/auth/login"] = map[string]any{"post": map[string]any{"tags": []string{"auth"}, "summary": "Вход", "requestBody": map[string]any{"required": true, "content": map[string]any{"application/x-www-form-urlencoded": map[string]any{"schema": map[string]any{"type": "object", "properties": map[string]any{"username": str("student@example.com"), "password": str("SiriusDemo2026!")}}}}}, "responses": map[string]any{"200": map[string]string{"description": "Пара токенов"}}}}

	add("get", "/menu/categories/{id}", "menu", nil, false)

	spec := map[string]any{"openapi": "3.0.3", "info": map[string]any{"title": "Сириус.Еда API", "version": "1.0.0"}, "paths": paths}

	spec["components"] = map[string]any{"securitySchemes": map[string]any{"oauth": map[string]any{"type": "oauth2", "flows": map[string]any{"password": map[string]any{"tokenUrl": "/auth/login", "scopes": map[string]string{}}}}}}

	return spec
}
func contains(s, part string) bool {
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
