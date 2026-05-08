package main

import (
	"context"
	"time"

	pb "outlook-imap-service/pb"
)

type emailServer struct {
	pb.UnimplementedEmailServiceServer
	accMgr  *AccountManager
	watcher *MailWatcher
}

func NewEmailServer(accMgr *AccountManager, watcher *MailWatcher) *emailServer {
	return &emailServer{
		accMgr:  accMgr,
		watcher: watcher,
	}
}

func (s *emailServer) GetEmail(ctx context.Context, req *pb.GetEmailRequest) (*pb.GetEmailResponse, error) {
	// We ignore the requested domain/prefix and use our own alias logic
	email := s.accMgr.GetNextEmail()
	return &pb.GetEmailResponse{EmailAddress: email}, nil
}

func (s *emailServer) WaitForEmail(ctx context.Context, req *pb.WaitForEmailRequest) (*pb.WaitForEmailResponse, error) {
	if content, ok := s.watcher.ConsumeCachedOTP(req.EmailAddress, req.SubjectKeyword); ok {
		return &pb.WaitForEmailResponse{Found: true, ContentExtracted: content}, nil
	}

	respChan := make(chan string, 1)

	s.watcher.AddWaiter(req.EmailAddress, req.SubjectKeyword, respChan)
	defer s.watcher.RemoveWaiter(req.EmailAddress)

	if content, ok := s.watcher.ConsumeCachedOTP(req.EmailAddress, req.SubjectKeyword); ok {
		return &pb.WaitForEmailResponse{Found: true, ContentExtracted: content}, nil
	}

	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	select {
	case content := <-respChan:
		return &pb.WaitForEmailResponse{Found: true, ContentExtracted: content}, nil
	case <-time.After(timeout):
		return &pb.WaitForEmailResponse{Found: false}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
