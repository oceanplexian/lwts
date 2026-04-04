# Login & Authentication

## Page Structure
- Navigate to `/login.html`. You should see a centered card with the LWTS logo and "Sign in to your workspace" subtitle. No board header visible.
- The login card should be no wider than 400px and centered on the page.

## Login Form
- You should see an email input and a password input with a show/hide toggle.
- Type a password, click the eye toggle. The password should become visible (input type changes to text). Click again to hide it.
- Click "Sign in" with empty fields. You should see validation errors.
- There should be a "Forgot password?" link and a "Create an account" link.

## Registration Form
- Click "Create an account". You should see 4 fields: name, email, password, confirm password.
- Click "Already have an account?" to go back to login.
- Type "ab" in the password field and blur. You should see "Weak" indicator in red.
- Type "Password1" — you should see "Fair".
- Type "MyP@ssw0rd123!" — you should see "Strong" in green.

## Forgot Password
- Click "Forgot password?" from login. You should see a single email input and a submit button.
- Enter any email and submit. You should see a success message.
- Click the back link to return to login.

## Form Validation
- Focus the email field, then blur without typing. You should see "Email is required".
- Type "notanemail" and blur. You should see an error about email format.
- Type "short" in the password field and blur. You should see an error about minimum length.
- Fill in different passwords in password and confirm. Blur confirm. You should see a mismatch error.
- After triggering an error, type a character in the field. The error should clear immediately.
- Error borders should use the red accent color.

## Submission States
- Fill in valid credentials and submit. While the request is in flight, you should see a spinner and all inputs should be disabled.
- If the server returns 401, you should see an error banner with a message. Click the X to dismiss it.

## Post-Login Redirect
- Log in with valid credentials. You should be redirected to `/` and `lwts_access_token` should exist in localStorage.
- Navigate to `/index.html` without a token. You should be redirected to `/login.html`.
- Log out. Navigate to `/index.html`. After logging back in, you should land on `/`.

## Logout
- Log in, then open Settings and click logout. You should be redirected to the login page. Both tokens should be cleared from localStorage.
