# Angular 18 — Enterprise Grade

> **Version**: 1.0.0 | **Stack**: Angular 18, Signals, standalone components, RxJS

```typescript
// auth.component.ts
import { Component, signal } from "@angular/core"
import { FormsModule } from "@angular/forms"

@Component({
  selector: "app-login",
  standalone: true,
  imports: [FormsModule],
  template: `
    <form (ngSubmit)="login()" class="flex flex-col gap-4 max-w-md">
      <input [(ngModel)]="email" name="email" type="email" placeholder="Email" />
      <input [(ngModel)]="password" name="password" type="password" placeholder="Password" />
      <p *ngIf="error()" class="text-red-500 text-sm">{{ error() }}</p>
      <button type="submit" [disabled]="loading()">
        {{ loading() ? "Loading..." : "Sign In" }}
      </button>
    </form>
  `,
})
export class LoginComponent {
  email = ""
  password = ""
  loading = signal(false)
  error = signal("")

  async login() {
    this.loading.set(true)
    try {
      const res = await fetch("/api/auth/login", { method: "POST", body: JSON.stringify({ email: this.email, password: this.password }) })
      if (!res.ok) throw new Error((await res.json()).message)
    } catch (e: any) {
      this.error.set(e.message)
    } finally {
      this.loading.set(false)
    }
  }
}
```

## Security

```bash
ng build --configuration production
ng test
npm audit
```

- Angular's built-in sanitizer prevents XSS in templates
- `DomSanitizer.bypassSecurityTrustHtml()` — audit every use
- Route guards (`canActivate`) for auth, never client-side only
- HttpClient interceptors for JWT injection
