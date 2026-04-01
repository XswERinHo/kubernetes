package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"kubernetes-manager/backend/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

var (
	mongoClient       *mongo.Client
	usersCollection   *mongo.Collection
	alertsCollection  *mongo.Collection
	historyCollection *mongo.Collection
)

func InitMongoDB(uri string) (*mongo.Database, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %v", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %v", err)
	}

	mongoClient = client
	db := client.Database("k8s_manager")
	usersCollection = db.Collection("users")
	alertsCollection = db.Collection("alerts")
	historyCollection = db.Collection("alert_history")
	log.Println("Connected to MongoDB!")

	ensureDefaultAdmin()
	EnsureDefaultAlerts()

	return db, nil
}

func ensureDefaultAdmin() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	count, err := usersCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		log.Printf("Error checking users count: %v", err)
		return
	}

	if count == 0 {
		log.Println("No users found. Creating default admin user...")
		hashedPassword, _ := hashPassword("password123")
		adminUser := models.User{
			Username: "admin",
			Password: hashedPassword,
			Role:     "Admin",
		}
		_, err := usersCollection.InsertOne(ctx, adminUser)
		if err != nil {
			log.Printf("Failed to create default admin: %v", err)
		} else {
			log.Println("Default admin created (admin / password123)")
		}
	}
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func AuthenticateUser(username, password string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var user models.User
	err := usersCollection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", false
		}
		log.Printf("Error finding user: %v", err)
		return "", false
	}

	if checkPasswordHash(password, user.Password) {
		return user.Role, true
	}
	return "", false
}

func CreateUser(username, password, role string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	count, err := usersCollection.CountDocuments(ctx, bson.M{"username": username})
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("user already exists")
	}

	hashedPassword, err := hashPassword(password)
	if err != nil {
		return err
	}

	newUser := models.User{
		Username: username,
		Password: hashedPassword,
		Role:     role,
	}

	_, err = usersCollection.InsertOne(ctx, newUser)
	return err
}

func ListUsers() ([]models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cursor, err := usersCollection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []models.User
	if err = cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func DeleteUser(username string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := usersCollection.DeleteOne(ctx, bson.M{"username": username})
	return err
}
