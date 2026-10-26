#!/usr/bin/env bash
# Scaffold for the review-merge-base case: builds a repository whose checked-out branch
# feat/login-throttle was cut from main before main gained two commits (a constant-time
# password comparison and SECURITY.md). Diffing against main's tip makes the branch look as if
# it removed both; the merge-base diff does not. The throttle's store, app/throttle_store.py,
# is left untracked although the committed app/throttle.py imports it. Runs under pipefail so
# a failed step stops the scaffold instead of leaving a half-built repository.
set -euo pipefail

git init -q -b main .
commit() { # commit <iso date> <author name> <author email> <message>
  GIT_AUTHOR_DATE="$1" GIT_COMMITTER_DATE="$1" git -c user.name="$2" -c user.email="$3" \
    -c commit.gpgsign=false -c core.hooksPath=/dev/null commit -q -m "$4"
}

mkdir -p app tests

cat > .gitignore <<'SEED_EOF'
__pycache__/
SEED_EOF

: > app/__init__.py

cat > app/users.py <<'SEED_EOF'
"""In-memory user accounts."""
import hashlib
import os
from dataclasses import dataclass


@dataclass
class User:
    username: str
    salt: bytes
    password_hash: str


_USERS: dict[str, User] = {}


def add_user(username: str, password: str) -> User:
    salt = os.urandom(16)
    digest = hashlib.pbkdf2_hmac("sha256", password.encode(), salt, 100_000).hex()
    user = User(username, salt, digest)
    _USERS[username] = user
    return user


def find_user(username: str) -> User | None:
    return _USERS.get(username)
SEED_EOF

cat > app/auth.py <<'SEED_EOF'
"""Password login."""
import hashlib

from app.users import find_user


def hash_password(password: str, salt: bytes) -> str:
    return hashlib.pbkdf2_hmac("sha256", password.encode(), salt, 100_000).hex()


def login(username: str, password: str) -> bool:
    user = find_user(username)
    if user is None:
        return False
    return hash_password(password, user.salt) == user.password_hash
SEED_EOF

cat > tests/test_auth.py <<'SEED_EOF'
import unittest

from app.auth import login
from app.users import add_user


class LoginTest(unittest.TestCase):
    def test_right_password(self):
        add_user("ana", "correct horse")
        self.assertTrue(login("ana", "correct horse"))

    def test_wrong_password(self):
        add_user("ben", "battery staple")
        self.assertFalse(login("ben", "nope"))


if __name__ == "__main__":
    unittest.main()
SEED_EOF

git add -A
commit "2026-10-01T09:00:00Z" "Priya Raman" "priya@example.com" "auth: password login with PBKDF2 hashes"

# The branch under review, cut from that commit.
git switch -q -c feat/login-throttle

cat > app/throttle.py <<'SEED_EOF'
"""Login throttling: refuse logins for an account after repeated failures (AUTH-19)."""
from app.throttle_store import FailureStore

MAX_FAILURES = 5
_store = FailureStore()


def allowed(username: str) -> bool:
    return not _store.blocked(username, MAX_FAILURES)


def record(username: str, ok: bool) -> None:
    if not ok:
        _store.add_failure(username)
SEED_EOF

cat > app/auth.py <<'SEED_EOF'
"""Password login."""
import hashlib

from app import throttle
from app.users import find_user


def hash_password(password: str, salt: bytes) -> str:
    return hashlib.pbkdf2_hmac("sha256", password.encode(), salt, 100_000).hex()


def login(username: str, password: str) -> bool:
    if not throttle.allowed(username):
        return False
    user = find_user(username)
    ok = user is not None and hash_password(password, user.salt) == user.password_hash
    throttle.record(username, ok)
    return ok
SEED_EOF

cat > tests/test_throttle.py <<'SEED_EOF'
import unittest

from app.auth import login
from app.users import add_user


class ThrottleTest(unittest.TestCase):
    def test_refuses_after_five_failures(self):
        add_user("cara", "s3cret pass")
        for _ in range(5):
            self.assertFalse(login("cara", "wrong"))
        self.assertFalse(login("cara", "s3cret pass"))


if __name__ == "__main__":
    unittest.main()
SEED_EOF

git add -A
commit "2026-10-06T14:20:00Z" "Sam Lee" "sam@example.com" "auth: throttle logins after repeated failures (AUTH-19)"

# Meanwhile on main: two commits the branch does not have.
git switch -q main

cat > app/auth.py <<'SEED_EOF'
"""Password login."""
import hashlib
import hmac

from app.users import find_user


def hash_password(password: str, salt: bytes) -> str:
    return hashlib.pbkdf2_hmac("sha256", password.encode(), salt, 100_000).hex()


def login(username: str, password: str) -> bool:
    user = find_user(username)
    if user is None:
        return False
    return hmac.compare_digest(hash_password(password, user.salt), user.password_hash)
SEED_EOF
git add -A
commit "2026-10-08T10:05:00Z" "Priya Raman" "priya@example.com" "auth: compare password hashes in constant time"

cat > SECURITY.md <<'SEED_EOF'
# Security policy

Report vulnerabilities to security@example.com. We reply within two business days.
SEED_EOF
git add -A
commit "2026-10-09T16:40:00Z" "Priya Raman" "priya@example.com" "docs: add security policy"

# Back to the branch under review. The store it imports was never added.
git switch -q feat/login-throttle

cat > app/throttle_store.py <<'SEED_EOF'
"""In-memory failure counts for the login throttle."""


class FailureStore:
    def __init__(self) -> None:
        self._failures: dict[str, int] = {}

    def add_failure(self, username: str) -> None:
        self._failures[username] = self._failures.get(username, 0) + 1

    def blocked(self, username: str, limit: int) -> bool:
        return self._failures.get(username, 0) >= limit
SEED_EOF
