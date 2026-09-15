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

// InitFirebaseAdmin initializes the Firebase Admin SDK with Auth and Firestore
func InitFirebaseAdmin() error {
	ctx := context.Background()

	// Try to get credentials from environment
	credPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	var opt option.ClientOption

	if credPath != "" {
		log.Printf("Using Firebase credentials from: %s", credPath)
		opt = option.WithCredentialsFile(credPath)
	} else {
		defaultPath := "./firebase-service-account.json"
		if _, err := os.Stat(defaultPath); err == nil {
			log.Printf("Using Firebase credentials from: %s", defaultPath)
			opt = option.WithCredentialsFile(defaultPath)
		} else {
			log.Println("No explicit Firebase credentials found, using Application Default Credentials")
			opt = nil
		}
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

// VerifiedUser contains verified identity information from Firebase
type VerifiedUser struct {
	UID         string
	Email       string
	DisplayName string
}

// VerifyIDToken verifies a Firebase ID token and returns verified user information
// The email and display name come from Firebase Auth (server-side), NOT from client headers
func VerifyIDToken(ctx context.Context, idToken string) (*VerifiedUser, error) {
	if authClient == nil {
		return nil, fmt.Errorf("Firebase Auth client not initialized")
	}

	token, err := authClient.VerifyIDToken(ctx, idToken)
	if err != nil {
		log.Printf("Firebase token verification failed: %v", err)
		return nil, fmt.Errorf("invalid or expired authentication token")
	}

	// Get verified email from token claims
	email, _ := token.Claims["email"].(string)

	// Try to get display name from Firebase Auth user record (server-side)
	// This is SAFE - comes from Firebase, not from client headers
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

// IsInitialized returns true if Firebase Admin is initialized
func IsInitialized() bool {
	return authClient != nil && firestoreClient != nil
}

// Close closes the Firestore client
func Close() error {
	if firestoreClient != nil {
		return firestoreClient.Close()
	}
	return nil
}
