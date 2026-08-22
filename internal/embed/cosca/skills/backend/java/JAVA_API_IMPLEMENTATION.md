# Java API — Enterprise Grade

> **Version**: 1.0.0 | **Stack**: Java 21, Spring Boot 3, JPA, Flyway, virtual threads

## Controller

```java
@RestController
@RequestMapping("/api/v1/users")
public class UserController {
    private final UserService service;

    @GetMapping("/{id}")
    public ResponseEntity<User> get(@PathVariable String id) {
        return service.findById(id)
            .map(ResponseEntity::ok)
            .orElse(ResponseEntity.notFound().build());
    }

    @PostMapping
    @PreAuthorize("hasRole('ADMIN')")
    public ResponseEntity<User> create(@Valid @RequestBody CreateUserInput input) {
        return ResponseEntity.status(201).body(service.create(input));
    }
}
```

## Validation

```java
public record CreateUserInput(
    @NotBlank @Size(min = 1, max = 100) String name,
    @Email @Size(max = 254) String email
) {}
```

## Security

```bash
./gradlew dependencyCheckAnalyze   # OWASP dependency check
./gradlew test
```

```java
// NEVER: String apiKey = "sk-...";
// Use: @Value("${api.key}") with external config
// Spring Security: method-level @PreAuthorize, CSRF protection enabled by default
// CORS: @CrossOrigin with explicit origins, not "*"
```
