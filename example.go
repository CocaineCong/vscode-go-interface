package main

import "fmt"

// TokenManager 接口定义
type TokenManager interface {
	AddToken(token string) error
	RemoveToken(token string) error
	ValidateToken(token string) bool
}

// SimpleTokenManager 实现
type SimpleTokenManager struct {
	tokens map[string]bool
}

// AddToken 方法实现
func (s *SimpleTokenManager) AddToken(token string) error {
	if s.tokens == nil {
		s.tokens = make(map[string]bool)
	}
	s.tokens[token] = true
	fmt.Printf("Token %s added\n", token)
	return nil
}

// RemoveToken 方法实现
func (s *SimpleTokenManager) RemoveToken(token string) error {
	delete(s.tokens, token)
	fmt.Printf("Token %s removed\n", token)
	return nil
}

// ValidateToken 方法实现
func (s *SimpleTokenManager) ValidateToken(token string) bool {
	return s.tokens[token]
}

type SimpleTokenManager2 struct {
	tokens map[string]bool
}

// AddToken 方法实现
func (s *SimpleTokenManager2) AddToken(token string) error {
	if s.tokens == nil {
		s.tokens = make(map[string]bool)
	}
	s.tokens[token] = true
	fmt.Printf("Token %s added\n", token)
	return nil
}

// RemoveToken 方法实现
func (s *SimpleTokenManager2) RemoveToken(token string) error {
	delete(s.tokens, token)
	fmt.Printf("Token %s removed\n", token)
	return nil
}

// ValidateToken 方法实现
func (s *SimpleTokenManager2) ValidateToken2(token string) bool {
	return s.tokens[token]
}

func main() {
	tm := &SimpleTokenManager{}
	tm.AddToken("abc123")
	fmt.Println(tm.ValidateToken("abc123"))
}
