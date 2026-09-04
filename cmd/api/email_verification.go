package main

import "net/http"

const verifyEmailPage = `<!doctype html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>Verify your email | TaskFlow</title>
</head>
<body>
	<main>
		<h1>Email verification</h1>
		<p id="status">Verifying your email address...</p>
	</main>

	<script>
		const status = document.getElementById("status");
		const token = new URLSearchParams(window.location.search).get("token");

		if (!token) {
			status.textContent = "This verification link is missing its token.";
		} else {
			fetch("/users/verify-email", {
				method: "POST",
				headers: {
					"Content-Type": "application/json"
				},
				body: JSON.stringify({ token: token })
			})
				.then(async function(response) {
					const data = await response.json().catch(function() {
						return {};
					});

					if (!response.ok) {
						throw new Error("This verification link is invalid or expired.");
					}

					// Remove the sensitive token from the visible browser URL.
					history.replaceState({}, "", "/verify-email");
					status.textContent = data.message;
				})
				.catch(function(error) {
					status.textContent = error.message;
				});
		}
	</script>
</body>
</html>
`

func (app *application) verifyEmailPageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if _, err := w.Write([]byte(verifyEmailPage)); err != nil {
		app.errorLog.Printf("write verification page: %v", err)
	}
}
