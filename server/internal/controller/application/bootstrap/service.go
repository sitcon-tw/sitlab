package bootstrap

import (
	"context"
	"fmt"

	appboard "example.com/project-template/internal/controller/application/board"
	appdirectory "example.com/project-template/internal/controller/application/directory"
	"example.com/project-template/internal/domain/directory"
	"example.com/project-template/internal/domain/identity"
)

type Result struct {
	Me          identity.User
	CSRFToken   string
	Directory   directory.Snapshot
	Board       appboard.Snapshot
	Preferences appdirectory.Preferences
	Sync        SyncStatus
}

type Service struct {
	auth      Auth
	directory Directory
	board     Board
	sync      Sync
}

func NewService(auth Auth, directory Directory, board Board, sync Sync) *Service {
	return &Service{auth: auth, directory: directory, board: board, sync: sync}
}

func (s *Service) Get(ctx context.Context, claims identity.SessionClaims) (Result, error) {
	me, err := s.auth.Me(ctx, claims.UserID)
	if err != nil {
		return Result{}, err
	}
	directorySnapshot, err := s.directory.Snapshot(ctx)
	if err != nil {
		return Result{}, err
	}
	boardSnapshot, err := s.board.Board(ctx)
	if err != nil {
		return Result{}, err
	}
	preferences, err := s.directory.Preferences(ctx, claims.UserID)
	if err != nil {
		return Result{}, err
	}
	status, err := s.sync.Status(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("load sync status: %w", err)
	}
	csrfToken, err := s.auth.IssueCSRF(ctx, claims)
	if err != nil {
		return Result{}, fmt.Errorf("issue bootstrap csrf token: %w", err)
	}
	return Result{
		Me: me, CSRFToken: csrfToken, Directory: directorySnapshot,
		Board: boardSnapshot, Preferences: preferences, Sync: status,
	}, nil
}
