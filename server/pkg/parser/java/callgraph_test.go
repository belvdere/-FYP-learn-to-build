package java

import (
	"testing"
)

// TestCallGraphDeduplication tests that duplicate method calls are removed
func TestCallGraphDeduplication(t *testing.T) {
	code := `
public class Example {
    public void process() {
        // These should not create duplicates
        logger.info("test");
        service.save(user);
        service.save(user);  // Same line, different call - should create 2 entries
    }
}`

	extractor := NewJavaCallGraphExtractor()
	calls, err := extractor.ExtractMethodCalls("Example.java", []byte(code))
	if err != nil {
		t.Fatalf("Failed to extract method calls: %v", err)
	}

	// Count unique caller:callee:line combinations
	seen := make(map[string]int)
	for _, call := range calls {
		key := call.CallerID + ":" + call.CalleeID + ":" + string(rune(call.Line))
		seen[key]++
	}

	// Each unique combination should appear exactly once
	for key, count := range seen {
		if count != 1 {
			t.Errorf("Key %s appeared %d times, expected 1 (deduplication failed)", key, count)
		}
	}
}

// TestCallGraphExtraction tests basic call graph extraction
func TestCallGraphExtraction(t *testing.T) {
	tests := []struct {
		name          string
		code          string
		expectedCalls int
		expectedCallees []string
	}{
		{
			name: "Simple method calls",
			code: `
public class UserService {
    private Logger logger;
    private Repository repo;
    
    public void save(User user) {
        logger.info("Saving user");
        repo.save(user);
        logger.info("Done");
    }
}`,
			expectedCalls: 3,
			expectedCallees: []string{"Logger.info", "Repository.save", "Logger.info"},
		},
		{
			name: "Chained method calls",
			code: `
public class Example {
    private List<String> items;

    public void process() {
        items.stream()
             .map(String::toUpperCase)
             .collect(Collectors.toList());
    }
}`,
			// Chained calls:
			// items.stream() -> tracked (List.stream)
			// .map() -> skipped (chained)
			// String::toUpperCase -> tracked as method_reference (String.toUpperCase)
			// .collect() -> skipped (chained)
			// Collectors.toList() -> tracked (static call inside collect)
			expectedCalls: 3,
			expectedCallees: []string{"List.stream", "String.toUpperCase", "Collectors.toList"},
		},
		{
			name: "Static method calls",
			code: `
public class MathUtils {
    public int calculate() {
        return Math.max(10, 20) + Math.min(5, 3);
    }
}`,
			expectedCalls: 2,
			expectedCallees: []string{"Math.max", "Math.min"},
		},
		{
			name: "Nested method calls",
			code: `
public class Example {
    public void test() {
        process(transform(getData()));
    }
}`,
			expectedCalls: 3,
			// Note: Order depends on AST traversal (inside-out for nested calls)
			expectedCallees: []string{"Example.getData", "Example.transform", "Example.process"},
		},
		{
			name: "No method calls",
			code: `
public class Empty {
    private int value = 42;
}`,
			expectedCalls: 0,
			expectedCallees: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewJavaCallGraphExtractor()
			calls, err := extractor.ExtractMethodCalls("test.java", []byte(tt.code))
			if err != nil {
				t.Fatalf("Failed to extract method calls: %v", err)
			}

			if len(calls) != tt.expectedCalls {
				t.Errorf("Expected %d calls, got %d", tt.expectedCalls, len(calls))
				for i, call := range calls {
					t.Logf("  Call %d: %s -> %s (line %d)", i, call.CallerID, call.CalleeID, call.Line)
				}
			}

			// Verify expected callees are present (order may vary)
			if len(tt.expectedCallees) > 0 && len(calls) == len(tt.expectedCallees) {
				foundCallees := make(map[string]bool)
				for _, call := range calls {
					foundCallees[call.CalleeID] = true
				}
				
				for _, expectedCallee := range tt.expectedCallees {
					if !foundCallees[expectedCallee] {
						t.Errorf("Expected callee '%s' not found. Got:", expectedCallee)
						for i, call := range calls {
							t.Logf("  Call %d: %s -> %s", i, call.CallerID, call.CalleeID)
						}
						break
					}
				}
			}
		})
	}
}

// TestCallGraphCallTypes tests call type detection (direct, static, super)
func TestCallGraphCallTypes(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		expectedType string
	}{
		{
			name: "Direct call",
			code: `
public class Example {
    void test() {
        service.save();
    }
}`,
			expectedType: "direct",
		},
		{
			name: "Static call",
			code: `
public class Example {
    void test() {
        Collections.sort(list);
    }
}`,
			expectedType: "static",
		},
		{
			name: "Super call",
			code: `
public class Child extends Parent {
    void test() {
        super.initialize();
    }
}`,
			expectedType: "super",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewJavaCallGraphExtractor()
	calls, err := extractor.ExtractMethodCalls("test.java", []byte(tt.code))
			if err != nil {
				t.Fatalf("Failed to extract method calls: %v", err)
			}

			if len(calls) == 0 {
				t.Fatal("Expected at least one call")
			}

			if calls[0].CallType != tt.expectedType {
				t.Errorf("Expected call type '%s', got '%s'", tt.expectedType, calls[0].CallType)
			}
		})
	}
}

// TestCallGraphContextTracking tests that caller context is correctly tracked
func TestCallGraphContextTracking(t *testing.T) {
	code := `
public class UserService {
    public void saveUser(User user) {
        repository.save(user);
    }
    
    public void deleteUser(String id) {
        repository.delete(id);
    }
}`

	extractor := NewJavaCallGraphExtractor()
	calls, err := extractor.ExtractMethodCalls("UserService.java", []byte(code))
	if err != nil {
		t.Fatalf("Failed to extract method calls: %v", err)
	}

	// Should have 2 calls from different methods
	if len(calls) != 2 {
		t.Fatalf("Expected 2 calls, got %d", len(calls))
	}

	// Both should have the class name in the caller
	for _, call := range calls {
		if call.CallerID != "UserService.saveUser" && call.CallerID != "UserService.deleteUser" {
			t.Errorf("Unexpected caller: %s", call.CallerID)
		}
	}

	// Check that different methods are tracked
	callers := make(map[string]bool)
	for _, call := range calls {
		callers[call.CallerID] = true
	}

	if len(callers) != 2 {
		t.Error("Expected calls from 2 different methods")
	}
}

// TestCallGraphNoDuplicatesOnReparse tests that re-parsing doesn't create duplicates
func TestCallGraphNoDuplicatesOnReparse(t *testing.T) {
	code := `
public class Example {
    public void test() {
        service.method();
    }
}`

	// Extract twice
	extractor := NewJavaCallGraphExtractor()
	calls1, err := extractor.ExtractMethodCalls("test.java", []byte(code))
	if err != nil {
		t.Fatalf("Failed first extraction: %v", err)
	}

	calls2, err := extractor.ExtractMethodCalls("test.java", []byte(code))
	if err != nil {
		t.Fatalf("Failed second extraction: %v", err)
	}

	// Should have same number of calls
	if len(calls1) != len(calls2) {
		t.Errorf("Different number of calls: %d vs %d", len(calls1), len(calls2))
	}

	// Each extraction should have the same calls
	if len(calls1) == 0 {
		t.Fatal("No calls extracted")
	}
}

// BenchmarkCallGraphExtraction benchmarks call graph extraction
func BenchmarkCallGraphExtraction(b *testing.B) {
	code := `
public class UserService {
    private Logger logger;
    private UserRepository repository;
    private EmailService emailService;
    
    public void createUser(User user) {
        logger.info("Creating user");
        repository.save(user);
        emailService.sendWelcomeEmail(user.getEmail());
        logger.info("User created");
    }
    
    public void updateUser(String id, User updates) {
        User existing = repository.findById(id);
        existing.update(updates);
        repository.save(existing);
        logger.info("User updated");
    }
    
    public void deleteUser(String id) {
        repository.deleteById(id);
        logger.info("User deleted");
    }
}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractor := NewJavaCallGraphExtractor()
		_, err := extractor.ExtractMethodCalls("UserService.java", []byte(code))
		if err != nil {
			b.Fatalf("Failed to extract: %v", err)
		}
	}
}

// TestTypeResolution tests that variable types are properly resolved to class names
func TestTypeResolution(t *testing.T) {
	tests := []struct {
		name            string
		code            string
		expectedCallee  string
		description     string
	}{
		{
			name: "Field type resolution",
			code: `
public class Service {
    private MongoTemplate mongoTemplate;
    
    public void test() {
        mongoTemplate.count(query);
    }
}`,
			expectedCallee:  "MongoTemplate.count",
			description:     "Should resolve field 'mongoTemplate' to class 'MongoTemplate'",
		},
		{
			name: "Local variable type resolution",
			code: `
public class Service {
    public void test() {
        TranslationService service = new TranslationService();
        service.translateSegment("text");
    }
}`,
			expectedCallee:  "TranslationService.translateSegment",
			description:     "Should resolve local variable 'service' to class 'TranslationService'",
		},
		{
			name: "Static method call preservation",
			code: `
public class Utils {
    public void test() {
        TextUtils.removeMarkdown("text");
    }
}`,
			expectedCallee:  "TextUtils.removeMarkdown",
			description:     "Should preserve static method calls as-is",
		},
		{
			name: "Field access path preservation",
			code: `
public class Example {
    public void test() {
        System.out.println("message");
    }
}`,
			expectedCallee:  "System.out.println",
			description:     "Should preserve field access paths like System.out",
		},
		{
			name: "Same-class method call",
			code: `
public class Calculator {
    public void calculate() {
        processData();
    }
    
    private void processData() {}
}`,
			expectedCallee:  "Calculator.processData",
			description:     "Should qualify methods in the same class",
		},
		{
			name: "Generic type resolution",
			code: `
public class Service {
    private List<String> items;
    
    public void test() {
        items.add("value");
    }
}`,
			expectedCallee:  "List.add",
			description:     "Should strip generic type parameters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewJavaCallGraphExtractor()
			calls, err := extractor.ExtractMethodCalls("test.java", []byte(tt.code))
			if err != nil {
				t.Fatalf("Failed to extract method calls: %v", err)
			}

			if len(calls) == 0 {
				t.Fatal("Expected at least one call")
			}

			// Find the relevant call (might be multiple in some test cases)
			found := false
			for _, call := range calls {
				if call.CalleeID == tt.expectedCallee {
					found = true
					t.Logf("✓ %s: %s -> %s", tt.description, call.CallerID, call.CalleeID)
					break
				}
			}

			if !found {
				t.Errorf("Expected callee '%s' not found. Got:", tt.expectedCallee)
				for i, call := range calls {
					t.Logf("  Call %d: %s -> %s", i, call.CallerID, call.CalleeID)
				}
				t.Errorf("%s - FAILED", tt.description)
			}
		})
	}
}

// TestChainedCallFiltering tests that chained method calls are properly filtered
func TestChainedCallFiltering(t *testing.T) {
	code := `
public class Example {
    public void test() {
        // Chained calls: only the first should be tracked
        Criteria.where("status").exists(true);
    }
}`

	extractor := NewJavaCallGraphExtractor()
	calls, err := extractor.ExtractMethodCalls("test.java", []byte(code))
	if err != nil {
		t.Fatalf("Failed to extract method calls: %v", err)
	}

	// Should only have the first call in the chain (where)
	if len(calls) != 1 {
		t.Errorf("Expected 1 call (only first in chain), got %d", len(calls))
		for i, call := range calls {
			t.Logf("  Call %d: %s -> %s", i, call.CallerID, call.CalleeID)
		}
	}

	// Verify it's the 'where' call
	if len(calls) > 0 && calls[0].CalleeID != "Criteria.where" {
		t.Errorf("Expected 'Criteria.where', got '%s'", calls[0].CalleeID)
	}
}

// TestComplexTypeResolution tests type resolution in more complex scenarios
func TestComplexTypeResolution(t *testing.T) {
	code := `
public class ApplicationStartupListener {
    private MongoTemplate mongoTemplate;
    private RedisConnectionFactory redisConnectionFactory;
    private TranslationService translationService;
    
    public void onApplicationEvent() {
        // Field type resolution
        long count = mongoTemplate.count(new Query(Criteria.where("status").exists(true)), "collection");
        
        // External library (preserved as-is)
        System.out.println("Count: " + count);
        
        // Another field type resolution
        String result = translationService.translateSegment("text");
    }
}

class TranslationService {
    public String translateSegment(String text) {
        // Static method call
        return TextUtils.removeMarkdown(text);
    }
}

class TextUtils {
    public static String removeMarkdown(String text) {
        return text;
    }
}`

	extractor := NewJavaCallGraphExtractor()
	calls, err := extractor.ExtractMethodCalls("test.java", []byte(code))
	if err != nil {
		t.Fatalf("Failed to extract method calls: %v", err)
	}

	// Expected calls (after type resolution and chained call filtering)
	expectedCallees := map[string]bool{
		"MongoTemplate.count":                true,  // Field type resolved
		"Query.<init>":                       true,  // Constructor call (new Query(...))
		"Criteria.where":                     true,  // Static call (first in chain)
		"System.out.println":                 true,  // Field access preserved
		"TranslationService.translateSegment": true, // Field type resolved
		"TextUtils.removeMarkdown":           true,  // Static call
	}

	t.Logf("Total calls extracted: %d", len(calls))
	for i, call := range calls {
		t.Logf("  Call %d: %s -> %s (line %d)", i, call.CallerID, call.CalleeID, call.Line)
	}

	// Verify all expected callees are present
	for _, call := range calls {
		if !expectedCallees[call.CalleeID] {
			t.Errorf("Unexpected callee: %s", call.CalleeID)
		}
	}

	// Verify we got all expected callees
	foundCallees := make(map[string]bool)
	for _, call := range calls {
		foundCallees[call.CalleeID] = true
	}

	for expectedCallee := range expectedCallees {
		if !foundCallees[expectedCallee] {
			t.Errorf("Missing expected callee: %s", expectedCallee)
		}
	}
}

