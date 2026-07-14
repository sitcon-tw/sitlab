namespace ProjectTemplate;

@example(#{
	id: "11111111-1111-1111-1111-111111111111",
	email: "owner@example.com",
	displayName: "Alex Morgan",
	createdAt: utcDateTime.fromISO("2026-07-10T08:00:00Z"),
	updatedAt: utcDateTime.fromISO("2026-07-10T08:00:00Z"),
})
model User {
	id: uuid;
	email: email;

	@minLength(1)
	@maxLength(64)
	displayName: string;

	createdAt: utcDateTime;
	updatedAt: utcDateTime;
}

@example(#{ email: "owner@example.com", displayName: "Alex Morgan", password: "correct-horse-battery-staple" })
model RegisterRequest {
	email: email;

	@minLength(1)
	@maxLength(64)
	displayName: string;

	@secret
	@minLength(12)
	@maxLength(128)
	password: string;
}

@example(#{ email: "owner@example.com", password: "correct-horse-battery-staple" })
model LoginRequest {
	email: email;

	@secret
	@minLength(12)
	@maxLength(128)
	password: string;
}

@example(#{
	user: #{
		id: "11111111-1111-1111-1111-111111111111",
		email: "owner@example.com",
		displayName: "Alex Morgan",
		createdAt: utcDateTime.fromISO("2026-07-10T08:00:00Z"),
		updatedAt: utcDateTime.fromISO("2026-07-10T08:00:00Z"),
	},
})
model AuthResponse {
	user: User;
}

@example(#{ token: "a-one-time-readable-csrf-token" })
model CSRFResponse {
	@secret
	token: string;
}
