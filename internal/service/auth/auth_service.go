package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/buharamanya/gophermart/internal/model"
	"github.com/buharamanya/gophermart/internal/repository"
	"github.com/buharamanya/gophermart/internal/util"
)

type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Register(ctx context.Context, login, password string) (string, error) {
	hash, err := util.HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	user := &model.User{
		Login:        login,
		PasswordHash: hash,
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		if errors.Is(err, repository.ErrUserExists) {
			return "", err
		}
		return "", fmt.Errorf("create user: %w", err)
	}

	return util.GenerateToken(user.ID)
}

func (s *Service) Login(ctx context.Context, login, password string) (string, error) {
	user, err := s.repo.GetUserByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return "", ErrInvalidCredentials
		}
		return "", fmt.Errorf("get user: %w", err)
	}

	match, err := util.CheckPasswordHash(password, user.PasswordHash)
	if err != nil {
		return "", fmt.Errorf("check password: %w", err)
	}
	if !match {
		return "", ErrInvalidCredentials
	}

	return util.GenerateToken(user.ID)
}

var ErrInvalidCredentials = errors.New("invalid credentials")
