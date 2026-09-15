package auth

import (
	"context"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

var (
	app             *firebase.App
	authClient      *auth.Client
	firestoreClient *firestore.Client
)

// InitFirebaseAdmin initializes the Firebase Admin SDK (Auth + Firestore)
//
// Credential sources (in priority order):
//  1. FIREBASE_CREDENTIALS env var (JSON string) — for cloud deploys
//  2. GOOGLE_APPLICATION_CREDENTIALS env var (file path) — for local dev
//  3. ./firebase-service-account.json (file) — for local dev
//  4. Application Default Credentials
func InitFirebaseAdmin() error {
	ctx := context.Background()

	var opt option.ClientOption

	// Priority 1: FIREBASE_CREDENTIALS env var (JSON string)
	if jsonCreds := os.Getenv("FIREBASE_CREDENTIALS"); jsonCreds != "" {
		log.Println("Using Firebase credentials from FIREBASE_CREDENTIALS env var")
		opt = option.WithCredentialsJSON([]byte(jsonCreds))
	} else if credPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); credPath != "" {
		// Priority 2: GOOGLE_APPLICATION_CREDENTIALS env var (file path)
		log.Printf("Using Firebase credentials from: %s", credPath)
		opt = option.WithCredentialsFile(credPath)
	} else if _, err := os.Stat("./firebase-service-account.json"); err == nil {
		// Priority 3: Local file
		log.Println("Using Firebase credentials from: ./firebase-service-account.json")
		opt = option.WithCredentialsFile("./firebase-service-account.json")
	} else {
		// Priority 4: Application Default Credentials
		log.Println("No explicit Firebase credentials found, using Application Default Credentials")
		opt = nil
	}

	var err error
	if opt != nil {
		app, err = firebase.NewApp(ctx, nil, opt)
	} else {
		app, err = firebase.NewApp(ctx, nil)
	}
	if err != nil {
		return fmt.Errorf("failed to initialize Firebase app: %w", err)
	}

	// Initialize Auth client
	authClient, err = app.Auth(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize Firebase Auth client: %w", err)
	}

	// Initialize Firestore client
	firestoreClient, err = app.Firestore(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize Firestore client: %w", err)
	}

	log.Println("✅ Firebase Admin SDK initialized (Auth + Firestore)")
	return nil
}

// VerifiedUser contains verified identity info from Firebase
type VerifiedUser struct {
	UID         string
	Email       string
	DisplayName string
}

// VerifyIDToken verifies a Firebase ID token and returns verified user information
func VerifyIDToken(ctx context.Context, idToken string) (*VerifiedUser, error) {
	if authClient == nil {
		return nil, fmt.Errorf("Firebase Auth client not initialized")
	}

	token, err := authClient.VerifyIDToken(ctx, idToken)
	if err != nil {
		log.Printf("Firebase token verification failed: %v", err)
		return nil, fmt.Errorf("invalid or expired authentication token")
	}

	email, _ := token.Claims["email"].(string)

	displayName := ""
	if userRecord, err := authClient.GetUser(ctx, token.UID); err == nil {
		displayName = userRecord.DisplayName
		if email == "" {
			email = userRecord.Email
		}
	}

	return &VerifiedUser{
		UID:         token.UID,
		Email:       email,
		DisplayName: displayName,
	}, nil
}

// GetAuthClient returns the Firebase Auth client
func GetAuthClient() *auth.Client {
	return authClient
}

// GetFirestoreClient returns the Firestore client
func GetFirestoreClient() *firestore.Client {
	return firestoreClient
}

// IsInitialized returns true if Firestore is initialized
func IsInitialized() bool {
	return firestoreClient != nil
}

// Close closes the Firestore client
func Close() error {
	if firestoreClient != nil {
		return firestoreClient.Close()
	}
	return nil
}
