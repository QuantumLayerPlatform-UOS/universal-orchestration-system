# Contributing to QLP-UOS

Thank you for your interest in contributing to the Query Language Processing - Unified Orchestration System! This document provides guidelines and best practices for contributing to the project.

## Development Workflow

### 1. Fork and Clone
1. Fork the repository to your GitHub account
2. Clone your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/QLP-UOS.git
   cd QLP-UOS
   ```
3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/ORIGINAL_OWNER/QLP-UOS.git
   ```

### 2. Feature Branch Workflow
1. Always work on a feature branch:
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/your-bug-fix
   ```
2. Keep your branch up to date:
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

### 3. Pull Request Process
1. Push your branch to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```
2. Create a Pull Request from your fork to the main repository
3. Fill out the PR template completely
4. Ensure all CI checks pass
5. Request review from maintainers
6. Address review feedback promptly
7. Once approved, the PR will be merged by maintainers

## Code Standards

### Go Standards
- Follow the official [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` for formatting (automatically enforced by CI)
- Use `golint` and `go vet` for linting
- Naming conventions:
  - Exported functions/types: `PascalCase`
  - Unexported functions/types: `camelCase`
  - Constants: `PascalCase` or `SCREAMING_SNAKE_CASE`
- Error handling:
  ```go
  if err != nil {
      return fmt.Errorf("failed to do something: %w", err)
  }
  ```
- Always defer cleanup operations
- Write table-driven tests where applicable

### Python Standards
- Follow [PEP 8](https://www.python.org/dev/peps/pep-0008/) style guide
- Use Black for formatting (automatically enforced by CI)
- Use type hints for all functions:
  ```python
  def process_data(input_data: List[Dict[str, Any]]) -> ProcessedResult:
      """Process input data and return results."""
      pass
  ```
- Docstrings for all public functions and classes (Google style)
- Naming conventions:
  - Functions/variables: `snake_case`
  - Classes: `PascalCase`
  - Constants: `SCREAMING_SNAKE_CASE`
- Use `pylint` and `mypy` for linting and type checking

### Node.js/TypeScript Standards
- Follow the [Airbnb JavaScript Style Guide](https://github.com/airbnb/javascript)
- Use Prettier for formatting (automatically enforced by CI)
- Use ESLint with the project configuration
- TypeScript specific:
  - Always use explicit types (avoid `any`)
  - Use interfaces for object shapes
  - Use enums for fixed sets of values
- Naming conventions:
  - Functions/variables: `camelCase`
  - Classes/interfaces: `PascalCase`
  - Constants: `SCREAMING_SNAKE_CASE`
- Use async/await instead of callbacks
- Handle errors appropriately with try/catch

### General Code Quality Standards
- Write self-documenting code
- Keep functions small and focused (single responsibility)
- Avoid deep nesting (max 3 levels)
- Use meaningful variable and function names
- Remove commented-out code
- No console.log/print statements in production code

## Testing Requirements

### Unit Tests
- Minimum 80% code coverage for new code
- Write tests for all public functions
- Use table-driven tests for multiple scenarios
- Mock external dependencies

### Go Testing
```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        // test cases
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

### Python Testing
```python
import pytest

class TestClassName:
    @pytest.mark.parametrize("input,expected", [
        # test cases
    ])
    def test_function_name(self, input, expected):
        # test implementation
        assert result == expected
```

### Node.js/TypeScript Testing
```typescript
describe('FunctionName', () => {
  it('should handle normal case', async () => {
    // test implementation
    expect(result).toBe(expected);
  });
});
```

### Integration Tests
- Test service interactions
- Test API endpoints with various inputs
- Test error scenarios
- Use test databases/services where applicable

## PR Checklist

Before submitting a PR, ensure:

- [ ] Code follows the style guidelines for the language
- [ ] Self-review of code performed
- [ ] Comments added for complex logic
- [ ] Documentation updated (if applicable)
- [ ] Tests added/updated
- [ ] All tests pass locally
- [ ] No new linting warnings
- [ ] Commit messages follow conventions
- [ ] Branch is up to date with main
- [ ] PR description clearly explains changes

## Commit Message Conventions

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

### Format
```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `test`: Test additions or modifications
- `build`: Build system changes
- `ci`: CI configuration changes
- `chore`: Other changes that don't affect src or test files

### Examples
```
feat(orchestrator): add query validation endpoint

- Add POST /api/v1/validate endpoint
- Implement query syntax validation
- Add comprehensive error messages

Closes #123
```

```
fix(parser): handle nested JSON queries correctly

Previously, nested JSON queries would fail with a parsing error.
This commit fixes the issue by properly handling recursion depth.

Fixes #456
```

## Code Review Guidelines

### For Authors
1. Keep PRs small and focused
2. Provide context in the PR description
3. Respond to feedback constructively
4. Update PR based on feedback promptly
5. Re-request review after making changes

### For Reviewers
1. Review promptly (within 24-48 hours)
2. Be constructive and specific
3. Suggest improvements, not just problems
4. Approve once concerns are addressed
5. Use GitHub review features:
   - ✅ Approve: Ready to merge
   - 💬 Comment: General feedback
   - 🔄 Request changes: Must be addressed

### Review Focus Areas
1. **Correctness**: Does the code do what it's supposed to?
2. **Design**: Is the code well-structured and maintainable?
3. **Performance**: Are there any performance concerns?
4. **Security**: Are there any security vulnerabilities?
5. **Testing**: Are tests comprehensive and meaningful?
6. **Documentation**: Is the code well-documented?

## Security Considerations

- Never commit secrets, API keys, or credentials
- Use environment variables for configuration
- Validate all inputs
- Sanitize outputs
- Follow OWASP guidelines
- Report security issues privately to maintainers

## Getting Help

- Create an issue for bugs or feature requests
- Join our community discussions
- Check existing issues before creating new ones
- Use clear, descriptive titles for issues and PRs

## License

By contributing, you agree that your contributions will be licensed under the same license as the project (MIT License).