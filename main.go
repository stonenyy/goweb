package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/netinternet/remoteaddr"
)

var logFile *os.File
var config Config

// init 函数在程序启动时初始化配置和日志文件
func init() {
	// 从命令行读取配置文件路径
	configPath := flag.String("config", "", "Path to config file")
	flag.Parse()
	if *configPath == "" {
		*configPath = "/root/mywebproject/config.json" // 默认配置文件路径
	}

	// 加载配置文件
	loadFile(*configPath)

	var err error
	logFile, err = os.OpenFile(loadConfig().LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	log.SetOutput(logFile) // 设置日志输出到文件
}

// Config 结构体用于存储配置文件中的配置项
type Config struct {
	CertFile string `json:"CertFile"` // TLS 证书文件路径
	KeyFile  string `json:"KeyFile"`  // TLS 私钥文件路径
	LogFile  string `json:"LogFile"`  // 日志文件路径
	RpAddr   string `json:"RpAddr"`   // 反向代理目标地址
	RpPath   string `json:"RpPath"`   // 反向代理路径
	CfHeader string `json:"CfHeader"` // 自定义请求头标识
}

// loadConfig
func loadConfig() Config {
	// 从配置文件或环境变量加载配置
	return Config{
		CertFile: config.CertFile,
		KeyFile:  config.KeyFile,
		LogFile:  config.LogFile,
		RpAddr:   config.RpAddr,
		RpPath:   config.RpPath,
		CfHeader: config.CfHeader,
	}
}

// loadFile 从指定路径加载配置文件
func loadFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		log.Fatal("Failed to open Config file:", err)
		return
	}
	defer file.Close() // 确保文件关闭

	// 解析 JSON 文件内容到 config 结构体
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		log.Fatal("解析 JSON 失败:", err)
		return
	}
}

// logFormat 格式化日志输出
func logFormat(tip string, uri string, ag string, ip string, cf string) {
	// 日志格式：{datetime|uri|user-agent|header|tip|ip}
	log.Printf("|%s|%s|%s|%s|%s|%s\n", time.Now().Format("2006/01/02 03:04:05 PM -0700"), uri, ag, cf, tip, ip)
}

// setupProxy 创建并返回一个反向代理
func setupProxy() *httputil.ReverseProxy {
	target, err := url.Parse(loadConfig().RpAddr)
	if err != nil {
		log.Fatal("Failed to parse target URL:", err)
	}

	return httputil.NewSingleHostReverseProxy(target)
}

// setupServer 创建并返回一个 HTTP 服务器
func setupServer(proxy *httputil.ReverseProxy) *http.Server {
	return &http.Server{
		Addr: ":443", // 监听 443 端口
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 解析客户端 IP 和端口
			ip, port := remoteaddr.Parse().IP(r)
			cf_header := r.Header.Get("x-flag")

			// 记录日志
			logFormat(r.RemoteAddr, r.RequestURI, r.UserAgent(), ip+":"+port, cf_header)

			// 检查请求头和路径是否符合条件
			if cf_header == loadConfig().CfHeader && r.URL.Path == loadConfig().RpPath {
				proxy.ServeHTTP(w, r)
			} else {
				// 返回 404 错误
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"error": "not found", "message": "The requested resource is not available"}`))
			}
		}),
		TLSConfig: &tls.Config{
			MinVersion:               tls.VersionTLS12,                         // 最低 TLS 版本
			CurvePreferences:         []tls.CurveID{tls.CurveP256, tls.X25519}, // 优先使用的曲线
			PreferServerCipherSuites: true,                                     // 优先使用服务器的加密套件
			NextProtos:               []string{"h2", "http/1.1"},               // 支持 HTTP/2
		},
		ReadTimeout:  5 * time.Second,   // 读取超时
		WriteTimeout: 10 * time.Second,  // 写入超时
		IdleTimeout:  120 * time.Second, // 空闲连接超时
	}
}

// main 函数是程序入口
func main() {

	proxy := setupProxy()        // 初始化反向代理
	server := setupServer(proxy) // 初始化 HTTP 服务器

	// 启动服务器使用https模式
	log.Println("Starting server tls on :443")
	if err := server.ListenAndServeTLS(loadConfig().CertFile, loadConfig().KeyFile); err != nil {
		log.Fatal("Server TLS error:", err)
	}
}
