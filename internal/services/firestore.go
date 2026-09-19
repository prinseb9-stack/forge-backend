package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"forge-backend/internal/models"
)

const (
	usersCollection      = "users"
	usageReservationsCol = "usageReservations"
	generationsCol       = "generations"
)

var (
	ErrUsageLimitExceeded = errors.New("usage limit exceeded")
	ErrReservationInvalid = errors.New("reservation invalid or not owned by user")
)

type FirestoreService struct {
	client *firestore.Client
}

func NewFirestoreService(client *firestore.Client) (*FirestoreService, error) {
	if client == nil {
		return nil, fmt.Errorf("Firestore client cannot be nil - Firebase Admin must be initialized")
	}
	return &FirestoreService{client: client}, nil
}

// ═══════════════════════════════════════════════════════════════════
// USER MANAGEMENT
// ═══════════════════════════════════════════════════════════════════

func (s *FirestoreService) GetOrCreateUser(ctx context.Context, uid, email, displayName string) (*models.User, error) {
	if s.client == nil {
		return nil, fmt.Errorf("Firestore client is not initialized")
	}
	if uid == "" {
		return nil, fmt.Errorf("uid is required")
	}

	docRef := s.client.Collection(usersCollection).Doc(uid)

	doc, err := docRef.Get(ctx)
	if err == nil {
		var user models.User
		if err := doc.DataTo(&user); err != nil {
			return nil, fmt.Errorf("failed to parse user data: %w", err)
		}

		if !user.Plan.IsValid() {
			log.Printf("🚨 CRITICAL: User %s has invalid plan '%s'", uid, user.Plan)
			return nil, fmt.Errorf("user %s has invalid plan '%s' - manual fix required", uid, user.Plan)
		}

		if user.UID == "" {
			user.UID = uid
		}

		return &user, nil
	}

	if status.Code(err) != codes.NotFound {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	log.Printf("Creating new user: %s", uid)

	now := time.Now().UTC()
	newUserDoc := map[string]interface{}{
		"uid":         uid,
		"email":       email,
		"displayName": displayName,
		"plan":        string(models.PlanFree),
		"textGeneration": map[string]interface{}{
			"usageCount":      0,
			"maxUsage":        models.DefaultMaxUsage(models.PlanFree),
			"periodStartedAt": now,
			"periodEndsAt":    now.Add(models.PeriodDuration),
		},
		"imageGeneration": map[string]interface{}{
			"usageCount":      0,
			"maxUsage":        models.DefaultImageMaxUsage(models.PlanFree),
			"periodStartedAt": now,
			"periodEndsAt":    now.Add(models.PeriodDuration),
		},
		"createdAt": firestore.ServerTimestamp,
		"updatedAt": firestore.ServerTimestamp,
	}

	_, err = docRef.Set(ctx, newUserDoc)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	doc, err = docRef.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read created user: %w", err)
	}

	var createdUser models.User
	if err := doc.DataTo(&createdUser); err != nil {
		return nil, fmt.Errorf("failed to parse created user: %w", err)
	}

	return &createdUser, nil
}

func (s *FirestoreService) GetUser(ctx context.Context, uid string) (*models.User, error) {
	if s.client == nil {
		return nil, fmt.Errorf("Firestore client is not initialized")
	}

	doc, err := s.client.Collection(usersCollection).Doc(uid).Get(ctx)
	if err != nil {
		return nil, err
	}

	var user models.User
	if err := doc.DataTo(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

// ═══════════════════════════════════════════════════════════════════
// TEXT GENERATION RESERVATIONS
// ═══════════════════════════════════════════════════════════════════

func (s *FirestoreService) ReserveTextGeneration(ctx context.Context, uid string) (string, *models.UsagePeriod, error) {
	return s.reserveUsage(ctx, uid, models.ReservationKindText,
		func(u *models.User) models.UsagePeriod { return u.TextGeneration },
		func(p models.UsagePeriod) map[string]interface{} {
			return map[string]interface{}{
				"textGeneration": map[string]interface{}{
					"usageCount":      p.UsageCount,
					"maxUsage":        p.MaxUsage,
					"periodStartedAt": p.PeriodStartedAt,
					"periodEndsAt":    p.PeriodEndsAt,
				},
				"updatedAt": firestore.ServerTimestamp,
			}
		},
		func(plan models.Plan) int { return models.DefaultMaxUsage(plan) },
		func(plan models.Plan) models.UsagePeriod { return models.NewUsagePeriod(plan) },
	)
}

func (s *FirestoreService) ConsumeTextGeneration(ctx context.Context, uid, reservationID string) error {
	return s.consumeUsage(ctx, uid, reservationID, models.ReservationKindText)
}

func (s *FirestoreService) RefundTextGeneration(ctx context.Context, uid, reservationID string) error {
	return s.refundUsage(ctx, uid, reservationID, models.ReservationKindText,
		func(u *models.User) models.UsagePeriod { return u.TextGeneration },
		func(p models.UsagePeriod) map[string]interface{} {
			return map[string]interface{}{
				"textGeneration": map[string]interface{}{
					"usageCount":      p.UsageCount,
					"maxUsage":        p.MaxUsage,
					"periodStartedAt": p.PeriodStartedAt,
					"periodEndsAt":    p.PeriodEndsAt,
				},
				"updatedAt": firestore.ServerTimestamp,
			}
		},
	)
}

// ═══════════════════════════════════════════════════════════════════
// IMAGE GENERATION RESERVATIONS
// ═══════════════════════════════════════════════════════════════════

func (s *FirestoreService) ReserveImageGeneration(ctx context.Context, uid string) (string, *models.UsagePeriod, error) {
	return s.reserveUsage(ctx, uid, models.ReservationKindImage,
		func(u *models.User) models.UsagePeriod { return u.ImageGeneration },
		func(p models.UsagePeriod) map[string]interface{} {
			return map[string]interface{}{
				"imageGeneration": map[string]interface{}{
					"usageCount":      p.UsageCount,
					"maxUsage":        p.MaxUsage,
					"periodStartedAt": p.PeriodStartedAt,
					"periodEndsAt":    p.PeriodEndsAt,
				},
				"updatedAt": firestore.ServerTimestamp,
			}
		},
		func(plan models.Plan) int { return models.DefaultImageMaxUsage(plan) },
		func(plan models.Plan) models.UsagePeriod { return models.NewImageUsagePeriod(plan) },
	)
}

func (s *FirestoreService) ConsumeImageGeneration(ctx context.Context, uid, reservationID string) error {
	return s.consumeUsage(ctx, uid, reservationID, models.ReservationKindImage)
}

func (s *FirestoreService) RefundImageGeneration(ctx context.Context, uid, reservationID string) error {
	return s.refundUsage(ctx, uid, reservationID, models.ReservationKindImage,
		func(u *models.User) models.UsagePeriod { return u.ImageGeneration },
		func(p models.UsagePeriod) map[string]interface{} {
			return map[string]interface{}{
				"imageGeneration": map[string]interface{}{
					"usageCount":      p.UsageCount,
					"maxUsage":        p.MaxUsage,
					"periodStartedAt": p.PeriodStartedAt,
					"periodEndsAt":    p.PeriodEndsAt,
				},
				"updatedAt": firestore.ServerTimestamp,
			}
		},
	)
}

// ═══════════════════════════════════════════════════════════════════
// GENERIC RESERVATION ENGINE
// ═══════════════════════════════════════════════════════════════════

type periodGetter func(u *models.User) models.UsagePeriod
type periodWriter func(p models.UsagePeriod) map[string]interface{}
type maxUsageFn func(plan models.Plan) int
type newPeriodFn func(plan models.Plan) models.UsagePeriod

func (s *FirestoreService) reserveUsage(
	ctx context.Context,
	uid string,
	kind models.ReservationKind,
	getPeriod periodGetter,
	writePeriod periodWriter,
	maxUsage maxUsageFn,
	newPeriod newPeriodFn,
) (string, *models.UsagePeriod, error) {
	if s.client == nil {
		return "", nil, fmt.Errorf("Firestore client is not initialized")
	}
	if uid == "" {
		return "", nil, fmt.Errorf("uid is required")
	}

	userRef := s.client.Collection(usersCollection).Doc(uid)
	var reservationID string
	var periodAfter models.UsagePeriod

	err := s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(userRef)
		if err != nil {
			return fmt.Errorf("failed to load user in transaction: %w", err)
		}

		var user models.User
		if err := doc.DataTo(&user); err != nil {
			return fmt.Errorf("failed to parse user: %w", err)
		}

		if !user.Plan.IsValid() {
			return fmt.Errorf("invalid plan: %s", user.Plan)
		}

		now := time.Now().UTC()
		period := getPeriod(&user)

		// Reset period if expired or missing
		if period.PeriodEndsAt.IsZero() || !now.Before(period.PeriodEndsAt) {
			period = newPeriod(user.Plan)
		}

		// Ensure maxUsage matches plan
		expectedMax := maxUsage(user.Plan)
		if period.MaxUsage != expectedMax {
			period.MaxUsage = expectedMax
		}

		// Unlimited plan: no reservation
		if period.MaxUsage == -1 {
			if err := tx.Set(userRef, writePeriod(period), firestore.MergeAll); err != nil {
				return err
			}
			periodAfter = period
			reservationID = ""
			return nil
		}

		// Zero-max plan: block (used for image on FREE)
		if period.MaxUsage == 0 {
			return ErrUsageLimitExceeded
		}

		// Check limit
		if period.UsageCount >= period.MaxUsage {
			return ErrUsageLimitExceeded
		}

		// Increment
		period.UsageCount++

		if err := tx.Set(userRef, writePeriod(period), firestore.MergeAll); err != nil {
			return err
		}

		// Create reservation
		reservationID = uuid.NewString()
		reservationRef := userRef.Collection(usageReservationsCol).Doc(reservationID)
		reservation := map[string]interface{}{
			"reservationId":   reservationID,
			"kind":            string(kind),
			"state":           string(models.ReservationStateReserved),
			"periodStartedAt": period.PeriodStartedAt,
			"periodEndsAt":    period.PeriodEndsAt,
			"createdAt":       firestore.ServerTimestamp,
			"updatedAt":       firestore.ServerTimestamp,
		}
		if err := tx.Set(reservationRef, reservation); err != nil {
			return err
		}

		periodAfter = period
		return nil
	})

	if err != nil {
		return "", nil, err
	}

	return reservationID, &periodAfter, nil
}

func (s *FirestoreService) consumeUsage(ctx context.Context, uid, reservationID string, kind models.ReservationKind) error {
	if reservationID == "" {
		return nil
	}
	if s.client == nil {
		return fmt.Errorf("Firestore client is not initialized")
	}

	userRef := s.client.Collection(usersCollection).Doc(uid)
	resRef := userRef.Collection(usageReservationsCol).Doc(reservationID)

	return s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(resRef)
		if err != nil {
			return fmt.Errorf("failed to load reservation: %w", err)
		}

		var res models.UsageReservation
		if err := doc.DataTo(&res); err != nil {
			return fmt.Errorf("failed to parse reservation: %w", err)
		}

		if res.Kind != kind {
			return ErrReservationInvalid
		}

		if res.State != models.ReservationStateReserved {
			return nil // idempotent
		}

		return tx.Set(resRef, map[string]interface{}{
			"state":     string(models.ReservationStateConsumed),
			"updatedAt": firestore.ServerTimestamp,
		}, firestore.MergeAll)
	})
}

func (s *FirestoreService) refundUsage(
	ctx context.Context,
	uid, reservationID string,
	kind models.ReservationKind,
	getPeriod periodGetter,
	writePeriod periodWriter,
) error {
	if reservationID == "" {
		return nil
	}
	if s.client == nil {
		return fmt.Errorf("Firestore client is not initialized")
	}

	userRef := s.client.Collection(usersCollection).Doc(uid)
	resRef := userRef.Collection(usageReservationsCol).Doc(reservationID)

	return s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		resDoc, err := tx.Get(resRef)
		if err != nil {
			return fmt.Errorf("failed to load reservation: %w", err)
		}

		var res models.UsageReservation
		if err := resDoc.DataTo(&res); err != nil {
			return fmt.Errorf("failed to parse reservation: %w", err)
		}

		if res.Kind != kind {
			return ErrReservationInvalid
		}

		if res.State != models.ReservationStateReserved {
			return nil // idempotent
		}

		userDoc, err := tx.Get(userRef)
		if err != nil {
			return fmt.Errorf("failed to load user: %w", err)
		}

		var user models.User
		if err := userDoc.DataTo(&user); err != nil {
			return fmt.Errorf("failed to parse user: %w", err)
		}

		period := getPeriod(&user)

		periodMatches := !period.PeriodStartedAt.IsZero() &&
			period.PeriodStartedAt.Equal(res.PeriodStartedAt)

		if periodMatches && period.UsageCount > 0 {
			period.UsageCount--
			if err := tx.Set(userRef, writePeriod(period), firestore.MergeAll); err != nil {
				return err
			}
		}

		return tx.Set(resRef, map[string]interface{}{
			"state":     string(models.ReservationStateRefunded),
			"updatedAt": firestore.ServerTimestamp,
		}, firestore.MergeAll)
	})
}

// ═══════════════════════════════════════════════════════════════════
// IMAGE GENERATION HISTORY
// ═══════════════════════════════════════════════════════════════════

// SaveImageGeneration writes metadata about a generated image to
// users/{uid}/generations/{id}. Failure here is non-fatal.
func (s *FirestoreService) SaveImageGeneration(
	ctx context.Context,
	uid, id, prompt, size, url string,
) error {
	if s.client == nil {
		return fmt.Errorf("Firestore client is not initialized")
	}
	if uid == "" || id == "" {
		return fmt.Errorf("uid and id are required")
	}

	docRef := s.client.
		Collection(usersCollection).
		Doc(uid).
		Collection(generationsCol).
		Doc(id)

	doc := map[string]interface{}{
		"id":        id,
		"kind":      string(models.GenerationKindImage),
		"prompt":    prompt,
		"model":     "agnes-image-2.1-flash",
		"url":       url,
		"size":      size,
		"createdAt": firestore.ServerTimestamp,
	}

	_, err := docRef.Set(ctx, doc)
	if err != nil {
		return fmt.Errorf("failed to save image generation: %w", err)
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════
// TEXT GENERATION HISTORY
// ═══════════════════════════════════════════════════════════════════

// SaveTextGeneration writes the text generation result to
// users/{uid}/generations/{id}. Failure here is non-fatal.
func (s *FirestoreService) SaveTextGeneration(
	ctx context.Context,
	uid, sourceContent string,
	results []PlatformResult,
) error {
	if s.client == nil {
		return fmt.Errorf("Firestore client is not initialized")
	}
	if uid == "" {
		return fmt.Errorf("uid is required")
	}

	genID := uuid.NewString()
	docRef := s.client.
		Collection(usersCollection).
		Doc(uid).
		Collection(generationsCol).
		Doc(genID)

	// Convert results to plain maps
	resultsData := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		resultsData = append(resultsData, map[string]interface{}{
			"platform": r.Platform,
			"content":  r.Content,
		})
	}

	doc := map[string]interface{}{
		"id":            genID,
		"kind":          string(models.GenerationKindText),
		"sourceContent": sourceContent,
		"results":       resultsData,
		"createdAt":     firestore.ServerTimestamp,
	}

	_, err := docRef.Set(ctx, doc)
	if err != nil {
		return fmt.Errorf("failed to save text generation: %w", err)
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════
// SUBSCRIPTION MANAGEMENT
// ═══════════════════════════════════════════════════════════════════

// SavePendingSubscription records a pending checkout so the webhook can
// identify which user + plan it belongs to.
func (s *FirestoreService) SavePendingSubscription(
	ctx context.Context,
	uid, txRef, planName, planID string,
	amount float64,
) error {
	if s.client == nil {
		return fmt.Errorf("Firestore client is not initialized")
	}

	docRef := s.client.
		Collection("pendingSubscriptions").
		Doc(txRef)

	doc := map[string]interface{}{
		"uid":       uid,
		"txRef":     txRef,
		"planName":  planName,
		"planID":    planID,
		"amount":    amount,
		"createdAt": firestore.ServerTimestamp,
	}

	_, err := docRef.Set(ctx, doc)
	if err != nil {
		return fmt.Errorf("failed to save pending subscription: %w", err)
	}
	return nil
}

// FindUserByTxRef looks up the pending subscription to find the user and plan
func (s *FirestoreService) FindUserByTxRef(ctx context.Context, txRef string) (string, string, error) {
	if s.client == nil {
		return "", "", fmt.Errorf("Firestore client is not initialized")
	}

	doc, err := s.client.
		Collection("pendingSubscriptions").
		Doc(txRef).
		Get(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to load pending subscription: %w", err)
	}

	var data struct {
		UID      string `firestore:"uid"`
		PlanName string `firestore:"planName"`
	}
	if err := doc.DataTo(&data); err != nil {
		return "", "", fmt.Errorf("failed to parse pending subscription: %w", err)
	}

	return data.UID, data.PlanName, nil
}

// ActivateSubscription updates the user's plan and stores the subscription details
func (s *FirestoreService) ActivateSubscription(
	ctx context.Context,
	uid string,
	sub models.Subscription,
) error {
	if s.client == nil {
		return fmt.Errorf("Firestore client is not initialized")
	}
	if uid == "" {
		return fmt.Errorf("uid is required")
	}

	userRef := s.client.Collection(usersCollection).Doc(uid)

	// Map plan name to internal Plan
	var newPlan models.Plan
	switch sub.PlanName {
	case "pro":
		newPlan = models.PlanPro
	case "higher_pro":
		newPlan = models.PlanHigherPro
	default:
		return fmt.Errorf("unknown plan name: %s", sub.PlanName)
	}

	// Set subscription timestamps
	sub.CreatedAt = time.Now().UTC()
	sub.UpdatedAt = time.Now().UTC()

	// Update user with new plan + subscription + reset usage periods
	update := map[string]interface{}{
		"plan":         string(newPlan),
		"subscription": sub,
		"textGeneration": map[string]interface{}{
			"usageCount":      0,
			"maxUsage":        models.DefaultMaxUsage(newPlan),
			"periodStartedAt": time.Now().UTC(),
			"periodEndsAt":    time.Now().UTC().Add(models.PeriodDuration),
		},
		"imageGeneration": map[string]interface{}{
			"usageCount":      0,
			"maxUsage":        models.DefaultImageMaxUsage(newPlan),
			"periodStartedAt": time.Now().UTC(),
			"periodEndsAt":    time.Now().UTC().Add(models.PeriodDuration),
		},
		"updatedAt": firestore.ServerTimestamp,
	}

	_, err := userRef.Set(ctx, update, firestore.MergeAll)
	if err != nil {
		return fmt.Errorf("failed to activate subscription: %w", err)
	}
	return nil
}

// ═══════════════════════════════════════════════════════════════════
// SCHEDULED POSTS
// ═══════════════════════════════════════════════════════════════════

const scheduledPostsCol = "scheduledPosts"

// CreateScheduledPost creates a new scheduled post
func (s *FirestoreService) CreateScheduledPost(
	ctx context.Context,
	uid, platform, content string,
	scheduledFor time.Time,
	notes string,
) (*models.ScheduledPost, error) {
	if s.client == nil {
		return nil, fmt.Errorf("Firestore client is not initialized")
	}
	if uid == "" {
		return nil, fmt.Errorf("uid is required")
	}

	postID := uuid.NewString()
	postRef := s.client.
		Collection(usersCollection).
		Doc(uid).
		Collection(scheduledPostsCol).
		Doc(postID)

	now := time.Now().UTC()
	post := models.ScheduledPost{
		ID:           postID,
		UID:          uid,
		Platform:     platform,
		Content:      content,
		ScheduledFor: scheduledFor.UTC(),
		Status:       models.ScheduleStatusPending,
		Notes:        notes,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	_, err := postRef.Set(ctx, post)
	if err != nil {
		return nil, fmt.Errorf("failed to save scheduled post: %w", err)
	}

	return &post, nil
}

// ListScheduledPosts returns all scheduled posts for a user,
// sorted by scheduledFor ascending.
func (s *FirestoreService) ListScheduledPosts(
	ctx context.Context,
	uid string,
) ([]models.ScheduledPost, error) {
	if s.client == nil {
		return nil, fmt.Errorf("Firestore client is not initialized")
	}
	if uid == "" {
		return nil, fmt.Errorf("uid is required")
	}

	iter := s.client.
		Collection(usersCollection).
		Doc(uid).
		Collection(scheduledPostsCol).
		OrderBy("scheduledFor", firestore.Asc).
		Documents(ctx)

	defer iter.Stop()

	var posts []models.ScheduledPost
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate scheduled posts: %w", err)
		}
		var p models.ScheduledPost
		if err := doc.DataTo(&p); err != nil {
			continue
		}
		// Only include pending + posted + failed, exclude cancelled
		if p.Status == models.ScheduleStatusCancelled {
			continue
		}
		posts = append(posts, p)
	}

	return posts, nil
}

// DeleteScheduledPost removes a scheduled post (user must own it)
func (s *FirestoreService) DeleteScheduledPost(
	ctx context.Context,
	uid, postID string,
) error {
	if s.client == nil {
		return fmt.Errorf("Firestore client is not initialized")
	}
	if uid == "" || postID == "" {
		return fmt.Errorf("uid and postID are required")
	}

	// Verify ownership: read the doc first, then delete
	postRef := s.client.
		Collection(usersCollection).
		Doc(uid).
		Collection(scheduledPostsCol).
		Doc(postID)

	doc, err := postRef.Get(ctx)
	if err != nil {
		return fmt.Errorf("post not found: %w", err)
	}

	var post models.ScheduledPost
	if err := doc.DataTo(&post); err != nil {
		return fmt.Errorf("failed to parse post: %w", err)
	}

	if post.UID != uid {
		return fmt.Errorf("post does not belong to user")
	}

	// Hard delete
	_, err = postRef.Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete scheduled post: %w", err)
	}

	return nil
}
