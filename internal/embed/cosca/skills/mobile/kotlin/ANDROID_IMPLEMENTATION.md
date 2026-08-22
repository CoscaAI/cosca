# Android Implementation — Enterprise Grade

> **Version**: 1.0.0 | **Status**: active | **Owner**: Mobile Chief | **Stack**: Kotlin 2.0+, Jetpack Compose, Android 14+

## Architecture — MVVM + Clean

```
app/
├── main/java/com/example/
│   ├── App.kt                         # Application class
│   ├── di/                            # Hilt/Koin modules
│   ├── core/
│   │   ├── network/                   # Retrofit + OkHttp
│   │   ├── security/                  # EncryptedSharedPrefs, Biometrics
│   │   └── extensions/
│   ├── feature/
│   │   ├── auth/
│   │   │   ├── AuthScreen.kt
│   │   │   └── AuthViewModel.kt
│   │   └── dashboard/
│   ├── shared/
│   │   ├── model/
│   │   └── component/
│   └── navigation/
```

## ViewModel + State

```kotlin
// feature/auth/AuthViewModel.kt
@HiltViewModel
class AuthViewModel @Inject constructor(
    private val authRepository: AuthRepository
) : ViewModel() {

    data class UiState(
        val email: String = "",
        val password: String = "",
        val isLoading: Boolean = false,
        val error: String? = null,
        val isSuccess: Boolean = false
    )

    private val _uiState = MutableStateFlow(UiState())
    val uiState: StateFlow<UiState> = _uiState.asStateFlow()

    fun onEmailChange(email: String) {
        _uiState.update { it.copy(email = email) }
    }

    fun onPasswordChange(password: String) {
        _uiState.update { it.copy(password = password) }
    }

    fun login() {
        val state = _uiState.value
        if (state.email.isBlank() || state.password.length < 8) return

        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, error = null) }

            authRepository.login(state.email, state.password)
                .onSuccess { user ->
                    _uiState.update { it.copy(isLoading = false, isSuccess = true) }
                }
                .onFailure { e ->
                    _uiState.update { it.copy(isLoading = false, error = e.message) }
                }
        }
    }
}
```

## Compose UI — All States

```kotlin
// feature/auth/AuthScreen.kt
@Composable
fun AuthScreen(
    viewModel: AuthViewModel = hiltViewModel(),
    onLoginSuccess: () -> Unit
) {
    val state by viewModel.uiState.collectAsStateWithLifecycle()

    LaunchedEffect(state.isSuccess) {
        if (state.isSuccess) onLoginSuccess()
    }

    AuthContent(
        email = state.email,
        password = state.password,
        isLoading = state.isLoading,
        error = state.error,
        onEmailChange = viewModel::onEmailChange,
        onPasswordChange = viewModel::onPasswordChange,
        onLoginClick = viewModel::login
    )
}

@Composable
private fun AuthContent(
    email: String,
    password: String,
    isLoading: Boolean,
    error: String?,
    onEmailChange: (String) -> Unit,
    onPasswordChange: (String) -> Unit,
    onLoginClick: () -> Unit
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(24.dp),
        verticalArrangement = Arrangement.Center
    ) {
        OutlinedTextField(
            value = email,
            onValueChange = onEmailChange,
            label = { Text("Email") },
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Email),
            singleLine = true,
            modifier = Modifier.fillMaxWidth()
        )

        Spacer(modifier = Modifier.height(16.dp))

        OutlinedTextField(
            value = password,
            onValueChange = onPasswordChange,
            label = { Text("Password") },
            visualTransformation = PasswordVisualTransformation(),
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
            singleLine = true,
            modifier = Modifier.fillMaxWidth()
        )

        if (error != null) {
            Text(error, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
        }

        Spacer(modifier = Modifier.height(24.dp))

        Button(
            onClick = onLoginClick,
            enabled = !isLoading,
            modifier = Modifier.fillMaxWidth()
        ) {
            if (isLoading) {
                CircularProgressIndicator(modifier = Modifier.size(20.dp))
            } else {
                Text("Sign In")
            }
        }
    }
}
```

## Retrofit + OkHttp

```kotlin
// core/network/ApiClient.kt
@Module
@InstallIn(SingletonComponent::class)
object NetworkModule {

    @Provides @Singleton
    fun provideOkHttp(tokenManager: TokenManager): OkHttpClient {
        return OkHttpClient.Builder()
            .connectTimeout(30, TimeUnit.SECONDS)
            .readTimeout(30, TimeUnit.SECONDS)
            .addInterceptor { chain ->
                val request = chain.request().newBuilder().apply {
                    tokenManager.getAccessToken()?.let {
                        addHeader("Authorization", "Bearer $it")
                    }
                }.build()
                chain.proceed(request)
            }
            .addInterceptor(HttpLoggingInterceptor().apply {
                level = if (BuildConfig.DEBUG) Level.BODY else Level.NONE
            })
            .build()
    }

    @Provides @Singleton
    fun provideRetrofit(okHttp: OkHttpClient): Retrofit {
        return Retrofit.Builder()
            .baseUrl(BuildConfig.API_BASE_URL)
            .client(okHttp)
            .addConverterFactory(MoshiConverterFactory.create(
                Moshi.Builder().add(KotlinJsonAdapterFactory()).build()
            ))
            .build()
    }
}

interface AuthApi {
    @POST("auth/login")
    suspend fun login(@Body request: LoginRequest): LoginResponse

    @POST("auth/refresh")
    suspend fun refresh(@Body request: RefreshRequest): LoginResponse
}
```

## Security — EncryptedSharedPrefs

```kotlin
// core/security/TokenManager.kt
@Singleton
class TokenManager @Inject constructor(
    @ApplicationContext private val context: Context
) {
    private val prefs = EncryptedSharedPreferences.create(
        "secure_prefs",
        MasterKeys.getOrCreate(MasterKeys.AES256_GCM_SPEC),
        context,
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
    )

    fun storeTokens(access: String, refresh: String) {
        prefs.edit()
            .putString("access_token", access)
            .putString("refresh_token", refresh)
            .apply()
    }

    fun getAccessToken(): String? = prefs.getString("access_token", null)

    fun getRefreshToken(): String? = prefs.getString("refresh_token", null)

    fun clearTokens() {
        prefs.edit().clear().apply()
    }
}
```

## Build & Deploy

```bash
# Build APK
./gradlew assembleRelease

# Build AAB (Play Store)
./gradlew bundleRelease

# Test
./gradlew test

# Lint
./gradlew lint

# Security scan
./gradlew dependencyCheckAnalyze
```

## Cosca Integration

```bash
cosca knowledge readiness --stack "compose,retrofit,hilt,room"
cosca knowledge search "Jetpack Compose state management"
```

## Security Checklist

- [ ] Tokens in EncryptedSharedPrefs, NEVER SharedPreferences
- [ ] ProGuard/R8 enabled with proper keep rules
- [ ] Certificate pinning via OkHttp CertificatePinner
- [ ] No `android:usesCleartextTraffic="true"` in production
- [ ] SafetyNet / Play Integrity attestation
- [ ] `android:debuggable="false"` in release
- [ ] No hardcoded API keys or secrets in code
- [ ] Biometric auth available (BiometricPrompt)
- [ ] Dependency audit (`./gradlew dependencyCheckAnalyze`)
