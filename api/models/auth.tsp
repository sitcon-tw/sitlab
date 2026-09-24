import "@typespec/http";

using Http;

namespace SitconBoard;

@example(#{
	id: "11111111-1111-1111-1111-111111111111",
	gitLabUserId: 123456,
	username: "yorukot",
	displayName: "Yorukot",
	avatarUrl: "https://gitlab.com/uploads/-/system/user/avatar/123456/avatar.png",
	profileUrl: "https://gitlab.com/yorukot",
	accessLevel: 40,
})
model CurrentUser {
	id: uuid;
	gitLabUserId: int64;

	@minLength(1)
	@maxLength(255)
	username: string;

	@minLength(1)
	@maxLength(255)
	displayName: string;

	@format("uri")
	avatarUrl: string | null;

	@format("uri")
	profileUrl: string;

	@minValue(0)
	@maxValue(100)
	accessLevel: int32;
}

@example(#{
	user: #{
		id: "11111111-1111-1111-1111-111111111111",
		gitLabUserId: 123456,
		username: "yorukot",
		displayName: "Yorukot",
		avatarUrl: null,
		profileUrl: "https://gitlab.com/yorukot",
		accessLevel: 40,
	},
})
model AuthResponse {
	user: CurrentUser;
}

@example(#{ token: "csrf-token-readable-by-the-browser-session" })
model CSRFResponse {
	@secret
	token: string;
}

model OAuthRedirect {
	...Response<302>;

	@header("Location")
	location: string;

	@header("Set-Cookie")
	setCookie?: string;
}

model GitLabOAuthCallbackQuery {
	@query
	@minLength(1)
	code: string;

	@query
	@minLength(1)
	state: string;
}

model MobileOAuthStartQuery {
	@query
	@minLength(43)
	@maxLength(128)
	codeChallenge: string;
}

model MobileOAuthCallbackQuery {
	@query
	code?: string;

	@query
	state?: string;

	@query
	error?: string;
}

model MobileOAuthFallbackResponse {
	...OkResponse;

	@header
	contentType: "text/html";

	@body
	body: string;
}

model MobileOAuthExchangeRequest {
	@minLength(1)
	code: string;

	@minLength(1)
	state: string;

	@minLength(43)
	@maxLength(128)
	codeVerifier: string;
}

model MobileOAuthExchangeResult {
	authenticated: true;
}

model MobileOAuthSessionResponse {
	...OkResponse;

	@header("Set-Cookie")
	setCookie: string;

	...Body<MobileOAuthExchangeResult>;
}
