package main // پکیج اصلی؛ برنامه از اینجا اجرا می‌شود

import (
	"context"       // برای مدیریت timeout و خاموش‌سازی امن (graceful shutdown)
	"encoding/json" // برای تبدیل داده‌ها به JSON
	"errors"        // برای بررسی نوع خطاها (errors.Is)
	"log"           // برای لاگ گرفتن
	"net/http"      // هسته HTTP در Go
	"os"            // خواندن متغیرهای محیطی مثل PORT
	"os/signal"     // دریافت سیگنال‌های سیستم
	"syscall"       // سیگنال‌های SIGINT و SIGTERM
	"time"          // زمان و timeout
)

// ================= Middleware =================

// نوع middleware: تابعی که handler می‌گیرد و handler جدید برمی‌گرداند
type Middleware func(http.Handler) http.Handler

// chain چند middleware را به ترتیب روی handler اصلی سوار می‌کند
func chain(h http.Handler, mws ...Middleware) http.Handler {
	// از آخر به اول middlewareها را wrap می‌کنیم
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h) // handler فعلی داخل middleware قرار می‌گیرد
	}
	return h // handler نهایی برگردانده می‌شود
}

// ================= Logging Middleware =================

// این middleware هر درخواست را لاگ می‌کند
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		start := time.Now() // زمان شروع رسیدگی به درخواست

		next.ServeHTTP(w, r) // ادامه‌ی مسیر به handler بعدی

		// لاگ نهایی بعد از پاسخ
		log.Printf(
			"%s %s %s (%s)",
			r.RemoteAddr,      // IP کلاینت
			r.Method,          // متد HTTP
			r.URL.Path,        // مسیر درخواست
			time.Since(start), // مدت زمان پاسخ
		)
	})
}

// ================= Recovery Middleware =================

// این middleware مانع از کرش سرور در صورت panic می‌شود
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// این defer حتی اگر panic رخ دهد اجرا می‌شود
		defer func() {
			if rec := recover(); rec != nil { // اگر panic رخ داده باشد
				log.Printf("panic recovered: %v", rec) // ثبت panic
				http.Error(
					w,
					"Internal Server Error",
					http.StatusInternalServerError,
				) // پاسخ 500
			}
		}()

		next.ServeHTTP(w, r) // ادامه‌ی اجرای درخواست
	})
}

// ================= Helper =================

// تابع کمکی برای ارسال پاسخ JSON
func writeJSON(w http.ResponseWriter, status int, v any) {

	// تعیین نوع خروجی
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// تنظیم status code
	w.WriteHeader(status)

	// تبدیل داده به JSON و ارسال
	_ = json.NewEncoder(w).Encode(v)
}

// ================= API Handlers =================

// /health → بررسی سلامت سرور
func healthHandler(w http.ResponseWriter, r *http.Request) {

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,                            // وضعیت سلامت
		"time": time.Now().Format(time.RFC3339), // زمان فعلی
	})
}

// /api/time → برگرداندن زمان
func apiTimeHandler(w http.ResponseWriter, r *http.Request) {

	writeJSON(w, http.StatusOK, map[string]any{
		"unix": time.Now().Unix(),               // زمان یونیکس
		"iso":  time.Now().Format(time.RFC3339), // زمان استاندارد
	})
}

// ================= main =================

func main() {

	// -------- Config --------

	// خواندن PORT از env
	port := os.Getenv("PORT")

	// اگر تعریف نشده بود، پیش‌فرض 8080
	if port == "" {
		port = "8080"
	}

	// -------- Router --------

	// ساخت router داخلی Go
	mux := http.NewServeMux()

	// ثبت routeهای API
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/time", apiTimeHandler)

	// وقتی کاربر / را می‌زند → index.html
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		// فقط دقیقاً مسیر / مجاز است
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		// ارسال فایل index.html
		http.ServeFile(w, r, "./static/index.html")
	})

	// سرو فایل‌های استاتیک مثل css, js, txt
	fs := http.FileServer(http.Dir("./static"))

	// /static/* → پوشه static
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// -------- Middleware --------

	// سوار کردن middlewareها روی router
	handler := chain(
		mux,                // handler اصلی
		recoveryMiddleware, // جلوگیری از panic
		loggingMiddleware,  // لاگ گرفتن
	)

	// -------- HTTP Server --------

	srv := &http.Server{
		Addr:              ":" + port,       // آدرس گوش دادن
		Handler:           handler,          // handler نهایی
		ReadTimeout:       5 * time.Second,  // timeout خواندن body
		ReadHeaderTimeout: 3 * time.Second,  // timeout header
		WriteTimeout:      10 * time.Second, // timeout پاسخ
		IdleTimeout:       60 * time.Second, // keep-alive
	}

	// -------- Start Server --------

	errCh := make(chan error, 1) // کانال دریافت خطا

	go func() {
		log.Printf("Server running on http://localhost:%s", port)
		errCh <- srv.ListenAndServe() // اجرای سرور
	}()

	// -------- Graceful Shutdown --------

	sigCh := make(chan os.Signal, 1)

	// گوش دادن به Ctrl+C و kill
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("Shutdown signal received: %s", sig)

	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Server error: %v", err)
		}
	}

	// ایجاد context با timeout برای خاموش‌سازی امن
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// خاموش‌سازی سرور
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	} else {
		log.Printf("Graceful shutdown complete.")
	}
}
