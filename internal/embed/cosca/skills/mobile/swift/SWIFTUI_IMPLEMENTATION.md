# SwiftUI Implementation — Enterprise Grade

> **Version**: 1.0.0 | **Status**: active | **Owner**: Mobile Chief | **Stack**: Swift 6, SwiftUI, iOS 17+

## Purpose

Production-grade iOS application with SwiftUI. Covers MVVM architecture, networking, persistence, navigation, state management, and App Store deployment.

## Architecture — MVVM

```
App/
├── App.swift                    # @main entry point
├── Core/
│   ├── Network/                 # URLSession, async/await
│   ├── Persistence/             # SwiftData / CoreData
│   ├── Security/                # Keychain, biometrics
│   └── Extensions/
├── Features/
│   ├── Auth/
│   │   ├── AuthView.swift
│   │   └── AuthViewModel.swift
│   └── Dashboard/
│       ├── DashboardView.swift
│       └── DashboardViewModel.swift
└── Shared/
    ├── Models/
    └── Components/
```

## ViewModel Pattern

```swift
// Features/Auth/AuthViewModel.swift
import SwiftUI

@MainActor
final class AuthViewModel: ObservableObject {
    @Published var email = ""
    @Published var password = ""
    @Published var isLoading = false
    @Published var errorMessage: String?

    private let authService: AuthServiceProtocol

    init(authService: AuthServiceProtocol = AuthService()) {
        self.authService = authService
    }

    var isFormValid: Bool {
        !email.isEmpty && email.contains("@") && password.count >= 8
    }

    func login() async {
        guard isFormValid else { return }

        isLoading = true
        errorMessage = nil

        do {
            let user = try await authService.login(email: email, password: password)
            // Store token in Keychain — NEVER UserDefaults
            try KeychainManager.shared.store(user.accessToken, for: .accessToken)
        } catch {
            errorMessage = error.localizedDescription
        }

        isLoading = false
    }
}
```

## View with All States

```swift
// Features/Auth/AuthView.swift
struct AuthView: View {
    @StateObject private var viewModel = AuthViewModel()

    var body: some View {
        NavigationStack {
            VStack(spacing: 24) {
                TextField("Email", text: $viewModel.email)
                    .textContentType(.emailAddress)
                    .keyboardType(.emailAddress)
                    .textInputAutocapitalization(.never)
                    .textFieldStyle(.roundedBorder)

                SecureField("Password", text: $viewModel.password)
                    .textContentType(.password)
                    .textFieldStyle(.roundedBorder)

                if let error = viewModel.errorMessage {
                    Text(error)
                        .foregroundColor(.red)
                        .font(.caption)
                }

                Button(action: { Task { await viewModel.login() } }) {
                    if viewModel.isLoading {
                        ProgressView()
                    } else {
                        Text("Sign In")
                    }
                }
                .buttonStyle(.borderedProminent)
                .disabled(!viewModel.isFormValid || viewModel.isLoading)
            }
            .padding()
            .navigationTitle("Login")
        }
    }
}
```

## Networking — async/await

```swift
// Core/Network/APIClient.swift
struct APIClient {
    static let shared = APIClient()
    private let session: URLSession
    private let decoder: JSONDecoder

    init() {
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = 30
        config.timeoutIntervalForResource = 60
        config.waitsForConnectivity = true
        session = URLSession(configuration: config)
        decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
    }

    func request<T: Decodable>(_ endpoint: Endpoint) async throws -> T {
        var request = URLRequest(url: endpoint.url)
        request.httpMethod = endpoint.method.rawValue
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")

        // Inject JWT from Keychain if available.
        if let token = try? KeychainManager.shared.get(.accessToken) {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }

        if let body = endpoint.body {
            request.httpBody = try JSONEncoder().encode(body)
        }

        let (data, response) = try await session.data(for: request)

        guard let http = response as? HTTPURLResponse else {
            throw APIError.invalidResponse
        }

        switch http.statusCode {
        case 200...299:
            return try decoder.decode(T.self, from: data)
        case 401:
            throw APIError.unauthorized
        case 429:
            // Respect Retry-After header
            let retryAfter = http.value(forHTTPHeaderField: "Retry-After")
            throw APIError.rateLimited(retryAfter: retryAfter)
        default:
            throw APIError.serverError(http.statusCode)
        }
    }
}
```

## Security — Keychain

```swift
// Core/Security/KeychainManager.swift
import Security

enum KeychainKey: String {
    case accessToken
    case refreshToken
}

final class KeychainManager {
    static let shared = KeychainManager()

    func store(_ value: String, for key: KeychainKey) throws {
        let data = Data(value.utf8)
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrAccount as String: key.rawValue,
            kSecValueData as String: data,
            kSecAttrAccessible as String: kSecAttrAccessibleWhenUnlockedThisDeviceOnly,
        ]
        // Delete existing item first to avoid duplicates.
        SecItemDelete(query as CFDictionary)

        let status = SecItemAdd(query as CFDictionary, nil)
        guard status == errSecSuccess else {
            throw KeychainError.failed(status: status)
        }
    }

    func get(_ key: KeychainKey) throws -> String {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrAccount as String: key.rawValue,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne,
        ]
        var result: AnyObject?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        guard status == errSecSuccess, let data = result as? Data else {
            throw KeychainError.notFound
        }
        return String(decoding: data, as: UTF8.self)
    }

    func delete(_ key: KeychainKey) {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrAccount as String: key.rawValue,
        ]
        SecItemDelete(query as CFDictionary)
    }
}
```

## Build & Deploy

```bash
# Build
xcodebuild -scheme App -configuration Release -sdk iphoneos

# Archive for App Store
xcodebuild archive -scheme App -archivePath build/App.xcarchive

# Test
xcodebuild test -scheme App -destination 'platform=iOS Simulator,name=iPhone 16'

# Lint
swiftlint lint --strict

# Security audit
# Check for hardcoded secrets, insecure URL schemes, missing ATS exceptions
```

## Cosca Integration

```bash
cosca knowledge readiness --stack "swiftui,keychain,async-await"
cosca knowledge search "SwiftUI MVVM pattern"
```

## Security Checklist

- [ ] Tokens in Keychain, NEVER UserDefaults
- [ ] `kSecAttrAccessibleWhenUnlockedThisDeviceOnly` for all secrets
- [ ] App Transport Security (ATS) enforced
- [ ] No hardcoded API keys or secrets
- [ ] Certificate pinning for sensitive endpoints
- [ ] Biometric auth available (Face ID / Touch ID)
- [ ] App sandbox enabled
- [ ] No `allowArbitraryLoads` in Info.plist
