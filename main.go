package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// --- DATABASE MODELS ---

type User struct {
	ID       uint   `gorm:"primaryKey"`
	Email    string `gorm:"unique"`
	Password string
	Role     string
	Employee Employee `gorm:"foreignKey:UserID"`
}

type Employee struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `json:"user_id"`
	NIP       string    `json:"nip"`
	Nama      string    `json:"nama"`
	Jabatan   string    `json:"jabatan"`
	Divisi    string    `json:"divisi"`
	CreatedAt time.Time `json:"created_at"`
}

type Attendance struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `json:"user_id"`
	Nama       string    `json:"nama"`
	Tanggal    string    `json:"tanggal" gorm:"column:tanggal"`
	Waktu      string    `json:"waktu"`
	Status     string    `json:"status"`
	Keterangan string    `json:"keterangan"`
	CreatedAt  time.Time `json:"created_at"`
}

type Notification struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `json:"user_id"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	IsRead    bool      `json:"is_read" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at"`
}

var db *gorm.DB
var jwtKey = []byte("rahasia_perusahaan_ini_harus_diganti")

// --- INIT DATABASE & SEEDER ---
func initDB() {
	var err error
	var dsn string

	// Ambil konfigurasi dari Environment Variables (Settingan Render)
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")

	// Cek apakah variabel DB_HOST ada (artinya sedang di Render)
	if dbHost != "" {
		// DSN untuk Production (Render -> TiDB)
		// Format: user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local&tls=true
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&tls=true",
			dbUser, dbPass, dbHost, dbPort, dbName)
	} else {
		// DSN untuk Localhost (Development di laptop)
		dsn = "root:@tcp(127.0.0.1:3306)/simpeg?charset=utf8mb4&parseTime=True&loc=Local"
	}

	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Gagal koneksi ke Database. Cek Env Variables atau XAMPP!")
	}

	db.AutoMigrate(&User{}, &Employee{}, &Attendance{}, &Notification{})

	var count int64
	db.Model(&User{}).Count(&count)
	if count == 0 {
		fmt.Println("Database kosong, menjalankan Seeder...")

		// 1. ADMIN
		admin := User{Email: "admin@kantor.com", Password: "123", Role: "admin"}
		db.Create(&admin)
		db.Create(&Employee{UserID: admin.ID, Nama: "Erina Fadillah", NIP: "ADM001", Jabatan: "HR Manager", Divisi: "HRD"})

		// 2. STAFF
		staff := User{Email: "staff@kantor.com", Password: "123", Role: "user"}
		db.Create(&staff)
		db.Create(&Employee{UserID: staff.ID, Nama: "Irham Akhbar", NIP: "STF001", Jabatan: "Staff Administrasi", Divisi: "General Affair"})

		// 3. TEKNISI
		teknisi := User{Email: "teknisi@kantor.com", Password: "123", Role: "user"}
		db.Create(&teknisi)
		db.Create(&Employee{UserID: teknisi.ID, Nama: "Rafly Adillah", NIP: "TEK001", Jabatan: "Teknisi Jaringan", Divisi: "IT Support"})

		// 4. SECURITY
		security := User{Email: "security@kantor.com", Password: "123", Role: "user"}
		db.Create(&security)
		db.Create(&Employee{UserID: security.ID, Nama: "Maulana Wasi", NIP: "SEC001", Jabatan: "Kepala Keamanan", Divisi: "Security"})

		fmt.Println("Seeder Berhasil! 4 Akun telah dibuat.")
	}
}

// --- HANDLERS ---

func Login(c *gin.Context) {
	var input User
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var user User
	if err := db.Preload("Employee").Where("email = ? AND password = ?", input.Email, input.Password).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email atau password salah"})
		return
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})
	tokenString, _ := token.SignedString(jwtKey)
	c.JSON(http.StatusOK, gin.H{
		"token": tokenString,
		"role":  user.Role,
		"nama":  user.Employee.Nama,
	})
}

func CreateUserEmployee(c *gin.Context) {
	type RegisterInput struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		NIP      string `json:"nip"`
		Nama     string `json:"nama"`
		Jabatan  string `json:"jabatan"`
		Divisi   string `json:"divisi"`
	}
	var input RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tx := db.Begin()
	newUser := User{Email: input.Email, Password: input.Password, Role: "user"}
	if err := tx.Create(&newUser).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Email sudah digunakan"})
		return
	}
	newEmp := Employee{UserID: newUser.ID, NIP: input.NIP, Nama: input.Nama, Jabatan: input.Jabatan, Divisi: input.Divisi}
	if err := tx.Create(&newEmp).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal simpan data pegawai"})
		return
	}
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"message": "User dan Pegawai berhasil dibuat"})
}

func SubmitAttendance(c *gin.Context) {
	var input Attendance
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := uint(c.MustGet("user_id").(float64))

	now := time.Now()
	today := now.Format("2006-01-02")

	// 1. CEK DUPLIKASI
	var count int64
	db.Model(&Attendance{}).Where("user_id = ? AND tanggal = ?", userID, today).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Anda sudah melakukan absensi hari ini. Tidak bisa absen 2 kali!"})
		return
	}

	var emp Employee
	db.Where("user_id = ?", userID).First(&emp)

	input.UserID = userID
	input.Nama = emp.Nama
	input.Tanggal = today
	input.Waktu = now.Format("15:04:05")

	if err := db.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal simpan absen"})
		return
	}

	// LOGIKA NOTIFIKASI TERLAMBAT
	batasWaktu := "08:00:00"
	if input.Status == "Hadir" && input.Waktu > batasWaktu {
		notif := Notification{
			UserID:  userID,
			Title:   "Anda Terlambat!",
			Message: fmt.Sprintf("Absen tercatat pukul %s (Batas: %s).", input.Waktu, batasWaktu),
			IsRead:  false,
		}
		db.Create(&notif)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Absen berhasil dicatat", "waktu": input.Waktu})
}

func GetNotifications(c *gin.Context) {
	userID := uint(c.MustGet("user_id").(float64))

	var notifs []Notification
	db.Where("user_id = ?", userID).Order("created_at desc").Limit(10).Find(&notifs)

	var unreadCount int64
	db.Model(&Notification{}).Where("user_id = ? AND is_read = ?", userID, false).Count(&unreadCount)

	c.JSON(http.StatusOK, gin.H{
		"data":         notifs,
		"total_unread": unreadCount,
	})
}

func MarkNotifRead(c *gin.Context) {
	userID := uint(c.MustGet("user_id").(float64))
	db.Model(&Notification{}).Where("user_id = ?", userID).Update("is_read", true)
	c.JSON(http.StatusOK, gin.H{"message": "All read"})
}

func GetAttendanceRecap(c *gin.Context) {
	role := c.MustGet("role").(string)
	userID := uint(c.MustGet("user_id").(float64))
	var history []Attendance
	currentMonth := time.Now().Format("2006-01")

	if role == "admin" {
		db.Where("tanggal LIKE ?", currentMonth+"%").Order("created_at desc").Find(&history)
	} else {
		db.Where("user_id = ?", userID).Order("created_at desc").Find(&history)
	}
	c.JSON(http.StatusOK, history)
}

func GetEmployees(c *gin.Context) {
	var employees []Employee
	db.Find(&employees)
	c.JSON(http.StatusOK, employees)
}

func GetDashboardStats(c *gin.Context) {
	var totalPegawai int64
	var totalHadir int64
	today := time.Now().Format("2006-01-02")

	// Logging Debugging akan sangat membantu di Render
	fmt.Println("--- DEBUG DASHBOARD ---")
	fmt.Println("Mencari Absensi Tanggal:", today)

	db.Model(&Employee{}).Count(&totalPegawai)
	err := db.Model(&Attendance{}).Where("tanggal = ? AND status = ?", today, "Hadir").Count(&totalHadir).Error
	if err != nil {
		fmt.Println("Error query absen:", err)
	}

	fmt.Println("Total Pegawai:", totalPegawai)
	fmt.Println("Total Hadir:", totalHadir)

	var persentase float64
	if totalPegawai > 0 {
		persentase = (float64(totalHadir) / float64(totalPegawai)) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"total_pegawai": totalPegawai,
		"total_hadir":   totalHadir,
		"persentase":    persentase,
	})
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if len(tokenString) > 7 {
			tokenString = tokenString[7:]
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token missing"})
			return
		}
		token, _ := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			c.Set("user_id", claims["user_id"])
			c.Set("role", claims["role"])
			c.Next()
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token invalid"})
		}
	}
}

func main() {
	initDB()
	r := gin.Default()

	config := cors.DefaultConfig()
	config.AllowAllOrigins = true // Aman untuk Single Deployment
	config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	r.Use(cors.New(config))

	// --- SETUP STATIC FILES (FRONTEND) ---
	// Melayani file statis dari folder "dist"
	// Pastikan folder "dist" hasil build React sudah dicopy ke sini
	r.Use(static.Serve("/", static.LocalFile("./dist", true)))

	// Handle SPA: Jika route tidak ditemukan (misal user refresh di /dashboard),
	// kembalikan index.html agar React Router yang menangani
	r.NoRoute(func(c *gin.Context) {
		c.File("./dist/index.html")
	})

	api := r.Group("/api")
	{
		api.POST("/login", Login)
		protected := api.Group("/")
		protected.Use(AuthMiddleware())
		{
			protected.POST("/users", CreateUserEmployee)
			protected.GET("/employees", GetEmployees)
			protected.GET("/dashboard-stats", GetDashboardStats)
			protected.POST("/attendance", SubmitAttendance)
			protected.GET("/attendance", GetAttendanceRecap)
			protected.GET("/notifications", GetNotifications)
			protected.POST("/notifications/read", MarkNotifRead)
		}
	}
	r.Run(":8080") // Render akan membaca PORT environment variable otomatis, 8080 adalah default fallback
}
