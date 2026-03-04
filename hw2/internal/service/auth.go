package service

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"hw2/internal/domain"
	"hw2/internal/repository"
)

var (
	ErrEmailTaken          = errors.New("email already taken")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrRefreshTokenInvalid = errors.New("refresh token invalid")
)

type AuthService struct {
	db              *pgxpool.Pool
	userRepo        *repository.UserRepo
	jwtSecret       []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func NewAuthService(db *pgxpool.Pool, userRepo *repository.UserRepo, jwtSecret string, accessTTL, refreshTTL time.Duration) *AuthService {
	return &AuthService{
		db:              db,
		userRepo:        userRepo,
		jwtSecret:       []byte(jwtSecret),
		accessTokenTTL:  accessTTL,
		refreshTokenTTL: refreshTTL,
	}
}

func (s *AuthService) Register(ctx context.Context, email, password, role string) (*domain.User, error) {
	_, err := s.userRepo.GetByEmail(ctx, s.db, email)
	if err == nil {
		return nil, ErrEmailTaken
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
	}
	if err := s.userRepo.Create(ctx, s.db, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, string, error) {
	user, err := s.userRepo.GetByEmail(ctx, s.db, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", "", ErrInvalidCredentials
		}
		return "", "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", "", ErrInvalidCredentials
	}

	_ = s.userRepo.DeleteExpiredTokens(ctx, s.db, user.ID)

	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return "", "", err
	}

	refreshToken := uuid.New().String()
	rt := &domain.RefreshToken{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(s.refreshTokenTTL),
	}
	if err := s.userRepo.SaveRefreshToken(ctx, s.db, rt); err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (string, string, error) {
	rt, err := s.userRepo.GetRefreshToken(ctx, s.db, refreshToken)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", "", ErrRefreshTokenInvalid
		}
		return "", "", err
	}

	if time.Now().After(rt.ExpiresAt) {
		_ = s.userRepo.DeleteRefreshToken(ctx, s.db, refreshToken)
		return "", "", ErrRefreshTokenInvalid
	}

	user, err := s.userRepo.GetByID(ctx, s.db, rt.UserID)
	if err != nil {
		return "", "", err
	}

	_ = s.userRepo.DeleteRefreshToken(ctx, s.db, refreshToken)

	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return "", "", err
	}

	newRefresh := uuid.New().String()
	newRT := &domain.RefreshToken{
		UserID:    user.ID,
		Token:     newRefresh,
		ExpiresAt: time.Now().Add(s.refreshTokenTTL),
	}
	if err := s.userRepo.SaveRefreshToken(ctx, s.db, newRT); err != nil {
		return "", "", err
	}

	return accessToken, newRefresh, nil
}

func (s *AuthService) generateAccessToken(user *domain.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id": user.ID.String(),
		"role":    user.Role,
		"exp":     time.Now().Add(s.accessTokenTTL).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) ValidateAccessToken(tokenString string) (uuid.UUID, string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return uuid.Nil, "", jwt.ErrTokenExpired
		}
		return uuid.Nil, "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return uuid.Nil, "", errors.New("invalid token")
	}

	userIDStr, _ := claims["user_id"].(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, "", errors.New("invalid user_id in token")
	}

	role, _ := claims["role"].(string)
	return userID, role, nil
}
