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
	Revision    string
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
	csrfToken, err := s.auth.IssueCSRF(ctx, claims)
	if err != nil {
		return Result{}, fmt.Errorf("issue bootstrap csrf token: %w", err)
	}
	for attempt := 0; attempt < 3; attempt++ {
		before, revisionErr := s.sync.Revision(ctx)
		if revisionErr != nil {
			return Result{}, fmt.Errorf("load bootstrap revision: %w", revisionErr)
		}
		directorySnapshot, snapshotErr := s.directory.Snapshot(ctx)
		if snapshotErr != nil {
			return Result{}, snapshotErr
		}
		boardSnapshot, boardErr := s.board.Board(ctx)
		if boardErr != nil {
			return Result{}, boardErr
		}
		preferences, preferencesErr := s.directory.Preferences(ctx, claims.UserID)
		if preferencesErr != nil {
			return Result{}, preferencesErr
		}
		status, statusErr := s.sync.Status(ctx)
		if statusErr != nil {
			return Result{}, fmt.Errorf("load sync status: %w", statusErr)
		}
		after, revisionErr := s.sync.Revision(ctx)
		if revisionErr != nil {
			return Result{}, fmt.Errorf("reload bootstrap revision: %w", revisionErr)
		}
		if before == after {
			return Result{
				Revision: after, Me: me, CSRFToken: csrfToken, Directory: directorySnapshot,
				Board: boardSnapshot, Preferences: preferences, Sync: status,
			}, nil
		}
	}
	return Result{}, fmt.Errorf("bootstrap changed while reading")
}
