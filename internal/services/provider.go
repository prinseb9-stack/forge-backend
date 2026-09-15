package services

type AIProvider interface {
	Generate(sourceContent string, platforms []string) (string, error)
}
