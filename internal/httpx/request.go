package httpx

import "net/http"

// RemoteAddr получает IP-адрес клиента, используя заголовок X-Real-IP, если он есть,
// иначе возвращает RemoteAddr из запроса.
func RemoteAddr(r *http.Request) string {
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	return r.RemoteAddr
}
