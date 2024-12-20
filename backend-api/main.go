package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"rosetta/models"
)

var client *mongo.Client

func main() {
	// Load environment variables
	databaseURL := os.Getenv("DATABASE_URL")

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	client, err = mongo.Connect(ctx, options.Client().ApplyURI(databaseURL))
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err = client.Disconnect(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	// Create a new router
	r := mux.NewRouter()

	// Define routes
	r.HandleFunc("/stories", createStory).Methods("POST")
	r.HandleFunc("/stories/{id}", deleteStory).Methods("DELETE")
	r.HandleFunc("/stories/{id}", updateStory).Methods("PUT")
	r.HandleFunc("/health", healthCheck).Methods("GET")

	// Start the server
	http.Handle("/", r)
	fmt.Println("Server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func createStory(w http.ResponseWriter, r *http.Request) {
	var story models.Story
	err := json.NewDecoder(r.Body).Decode(&story)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Initialize Segments to an empty array if it's nil
	if story.Segments == nil {
		story.Segments = []models.Segment{}
	}

	story.ID = primitive.NewObjectID()
	story.CreatedAt = time.Now()
	collection := client.Database("rosetta").Collection("stories")
	_, err = collection.InsertOne(context.Background(), story)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(story)
}

func deleteStory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.Error(w, "Invalid story ID", http.StatusBadRequest)
		return
	}

	collection := client.Database("rosetta").Collection("stories")
	_, err = collection.DeleteOne(context.Background(), bson.M{"_id": objectID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func updateStory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.Error(w, "Invalid story ID", http.StatusBadRequest)
		return
	}

	var story models.Story
	err = json.NewDecoder(r.Body).Decode(&story)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Ensure segments have IDs
	for i, segment := range story.Segments {
		if segment.ID.IsZero() {
			story.Segments[i].ID = primitive.NewObjectID()
		}
	}

	collection := client.Database("rosetta").Collection("stories")
	update := bson.M{
		"$set": bson.M{
			"title":        story.Title,
			"segments":     story.Segments,
			"is_published": story.IsPublished,
		},
	}

	_, err = collection.UpdateOne(context.Background(), bson.M{"_id": objectID}, update)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var updatedStory models.Story
	err = collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&updatedStory)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(updatedStory)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
