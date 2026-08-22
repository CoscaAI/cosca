# C# .NET API — Enterprise Grade

> **Version**: 1.0.0 | **Stack**: .NET 9, ASP.NET Core, EF Core, MediatR

## Minimal API

```csharp
var builder = WebApplication.CreateBuilder(args);
builder.Services.AddDbContext<AppDb>(o => o.UseSqlite(builder.Configuration.GetConnectionString("Default")));

var app = builder.Build();
app.UseHttpsRedirection();
app.UseAuthentication();
app.UseAuthorization();

app.MapGet("/health", () => Results.Ok(new { status = "ok" }));

app.MapGet("/api/v1/users/{id}", async (string id, AppDb db) =>
    await db.Users.FindAsync(id) is { } user ? Results.Ok(user) : Results.NotFound());

app.MapPost("/api/v1/users", async (CreateUserInput input, AppDb db) => {
    var user = new User { Name = input.Name, Email = input.Email };
    db.Users.Add(user); await db.SaveChangesAsync();
    return Results.Created($"/users/{user.Id}", user);
}).RequireAuthorization();

app.Run();
```

## Security

```bash
dotnet build -c Release
dotnet test
dotnet list package --vulnerable
```

```csharp
// NEVER: var key = "sk-...";
// Use: builder.Configuration["ApiKey"] with User Secrets / Azure Key Vault
// [Authorize(Roles = "Admin")] on sensitive endpoints
// Antiforgery: [ValidateAntiForgeryToken] on POST/PUT/DELETE
```
