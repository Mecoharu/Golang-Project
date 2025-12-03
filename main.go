package main

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

//  Tables 
type User struct {
	gorm.Model
	Name     string
	Email    string `gorm:"unique"`
	Password string
	Recipes  []Recipe
}

type Recipe struct {
	gorm.Model
	Title        string
	Description  string
	Ingredients  string
	Instructions string
	Image        string
	UserID       uint
	User         User
}
// One to Many relationship , satu user bisa punya banyak resep 

var db *gorm.DB

// DB 
func initDB() {
	var err error
	db, err = gorm.Open(sqlite.Open("recipes.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database")
	}
	db.AutoMigrate(&User{}, &Recipe{})
}


func main() {
	initDB()
	r := gin.Default()


	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("mysession", store))

	r.SetFuncMap(template.FuncMap{
		"lower": strings.ToLower,
	})

	
	r.LoadHTMLGlob("templates/*")
	
	
	r.Static("/uploads", "./uploads")

	// Routes
	// Public Routes (Guest bisa akses)
	r.GET("/", showDashboard)
	r.GET("/recipe/:id", showRecipeDetail) 
	
	// Auth Routes (Buat Admin/User)
	r.GET("/login", showLogin)
	r.POST("/login", login)
	r.GET("/register", showRegister)
	r.POST("/register", register)
	r.POST("/logout", logout)

	// Autorize Routes (Hanya untuk Admin /user yg login)
	authorized := r.Group("/")
	authorized.Use(authMiddleware())
	{
		authorized.GET("/create", showCreateRecipe)
		authorized.POST("/create", createRecipe)
		authorized.GET("/edit/:id", showEditRecipe)    
		authorized.POST("/edit/:id", updateRecipe)     
		authorized.POST("/delete/:id", deleteRecipe)
	}

	r.Run(":8080")
}


func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}



func showDashboard(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	
	var user User
	isLoggedIn := false
	
	if userID != nil {
		db.First(&user, userID)
		isLoggedIn = true
	}

	var recipes []Recipe
	db.Preload("User").Order("created_at desc").Find(&recipes)

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"User":       user,
		"Recipes":    recipes,
		"IsLoggedIn": isLoggedIn,
	})
}


func showRecipeDetail(c *gin.Context) {
	id := c.Param("id")
	
	session := sessions.Default(c)
	userID := session.Get("user_id")
	
	var user User
	isLoggedIn := false
	
	if userID != nil {
		db.First(&user, userID)
		isLoggedIn = true
	}

	var recipe Recipe
	if err := db.Preload("User").First(&recipe, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.HTML(http.StatusOK, "detail.html", gin.H{
		"User":       user,
		"Recipe":     recipe,
		"IsLoggedIn": isLoggedIn,
	})
}

func showCreateRecipe(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	var user User
	db.First(&user, userID)

	c.HTML(http.StatusOK, "create.html", gin.H{
		"User": user,
	})
}

// function form edit
func showEditRecipe(c *gin.Context) {
	id := c.Param("id")
	session := sessions.Default(c)
	userID := session.Get("user_id")
	
	var user User
	db.First(&user, userID)

	var recipe Recipe
	if err := db.First(&recipe, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/")
		return
	}


	if recipe.UserID != user.ID {
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.HTML(http.StatusOK, "edit.html", gin.H{
		"User":   user,
		"Recipe": recipe,
	})
}

//Create
func createRecipe(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id").(uint)

	file, err := c.FormFile("image")
	var imagePath string
	if err == nil {
		filename := filepath.Base(file.Filename)
		imagePath = "uploads/" + filename
		if err := c.SaveUploadedFile(file, imagePath); err != nil {
			c.String(http.StatusBadRequest, "Upload failed")
			return
		}
	}

	recipe := Recipe{
		Title:        c.PostForm("title"),
		Description:  c.PostForm("description"),
		Ingredients:  c.PostForm("ingredients"),
		Instructions: c.PostForm("instructions"),
		Image:        imagePath,
		UserID:       userID,
	}
	db.Create(&recipe)

	c.Redirect(http.StatusFound, "/")
}

// Handler untuk update resep
func updateRecipe(c *gin.Context) {
	id := c.Param("id")
	session := sessions.Default(c)
	userID := session.Get("user_id").(uint)

	var recipe Recipe
	if err := db.First(&recipe, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/")
		return
	}

	
	if recipe.UserID != userID {
		c.Redirect(http.StatusFound, "/")
		return
	}

	// Update
	recipe.Title = c.PostForm("title")
	recipe.Description = c.PostForm("description")
	recipe.Ingredients = c.PostForm("ingredients")
	recipe.Instructions = c.PostForm("instructions")


	file, err := c.FormFile("image")
	if err == nil {
		filename := filepath.Base(file.Filename)
		imagePath := "uploads/" + filename
		if err := c.SaveUploadedFile(file, imagePath); err == nil {
			recipe.Image = imagePath
		}
	}

	db.Save(&recipe)
	c.Redirect(http.StatusFound, "/recipe/"+id)
}

func deleteRecipe(c *gin.Context) {
	id := c.Param("id")
	db.Delete(&Recipe{}, id)
	c.Redirect(http.StatusFound, "/")
}

// AUTH HANDLERS
func showLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "auth.html", gin.H{"Type": "Login"})
}

func showRegister(c *gin.Context) {
	c.HTML(http.StatusOK, "auth.html", gin.H{"Type": "Register"})
}

func register(c *gin.Context) {
	password, _ := bcrypt.GenerateFromPassword([]byte(c.PostForm("password")), 14)
	user := User{
		Name:     c.PostForm("name"),
		Email:    c.PostForm("email"),
		Password: string(password),
	}
	result := db.Create(&user)
	if result.Error != nil {
		c.HTML(http.StatusOK, "auth.html", gin.H{"Type": "Register", "Error": "Email already exists"})
		return
	}
	c.Redirect(http.StatusFound, "/login")
}

func login(c *gin.Context) {
	var user User
	if err := db.Where("email = ?", c.PostForm("email")).First(&user).Error; err != nil {
		c.HTML(http.StatusOK, "auth.html", gin.H{"Type": "Login", "Error": "User not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(c.PostForm("password"))); err != nil {
		c.HTML(http.StatusOK, "auth.html", gin.H{"Type": "Login", "Error": "Wrong password"})
		return
	}

	session := sessions.Default(c)
	session.Set("user_id", user.ID)
	session.Save()

	c.Redirect(http.StatusFound, "/")
}

func logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.Redirect(http.StatusFound, "/login")
}
