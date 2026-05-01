# Adapter JVM

Nomos adapter for JVM-based projects (Java, Kotlin) with Spring Boot as the
primary framework target.

## Conventions Covered

### Detection Targets

| Annotation / Pattern       | Surface   | Capability                  |
|----------------------------|-----------|-----------------------------|
| `@RestController`          | api       | route_detection             |
| `@Controller`             | api       | route_detection             |
| `@RequestMapping` et al.  | api       | route_detection             |
| `@Service`                | api/worker| service_detection           |
| `@Component`              | worker    | service_detection           |
| `@Entity`                 | data      | data_model_detection        |
| `@Repository`             | data      | data_model_detection        |
| Hardcoded string catalogs  | api/data  | hardcoded_catalog_detection |

### Route Detection

Detects HTTP endpoints declared via Spring MVC annotations:

- `@GetMapping`, `@PostMapping`, `@PutMapping`, `@DeleteMapping`, `@PatchMapping`
- `@RequestMapping` with method attribute
- Class-level `@RequestMapping` prefix combined with method-level paths

### Service Detection

Detects Spring-managed service beans:

- `@Service` annotated classes
- `@Component` annotated classes used as business logic delegates

### Data Model Detection

Detects JPA/Hibernate persistence layer:

- `@Entity` annotated classes (maps to `data` surface)
- `@Repository` interfaces (Spring Data repositories)
- DTO classes by naming convention (`*DTO`, `*Dto`, `*Request`, `*Response`)

### Hardcoded Catalog Detection

Identifies potential hardcoded catalogs that should be externalized:

- Static `Map` or `List` fields with literal string values
- Enum types with display labels embedded
- `switch`/`if-else` chains mapping string keys to string values

## File Structure

```
adapters/jvm/
  adapter.nomos.yaml    # Adapter manifest (contract)
  README.md             # This file
  fixtures/
    spring-rest-service/  # Official test fixture
      pom.xml
      src/main/java/com/example/demo/
        controller/       # @RestController samples
        service/          # @Service samples
        entity/           # @Entity samples
        repository/       # @Repository samples
        dto/              # DTO samples
```

## Supported Stack

- **Languages**: Java (8+), Kotlin (1.6+)
- **Frameworks**: Spring Boot, Spring MVC, Spring WebFlux
- **Build tools**: Maven, Gradle (Groovy & Kotlin DSL)
- **Surfaces**: api, data, worker, batch

## Limitations

- Detection is annotation-driven; programmatic route registration is not covered.
- Multi-module projects should run the adapter per module for accurate results.
