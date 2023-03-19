import secrets
from hashlib import sha512

from starlette.applications import Starlette
from starlette.authentication import AuthCredentials, AuthenticationBackend, SimpleUser
from starlette.middleware import Middleware
from starlette.middleware.authentication import AuthenticationMiddleware
from starlette.requests import Request
from starlette.responses import (
    FileResponse,
    JSONResponse,
    RedirectResponse,
)
from starlette.routing import Mount, Route, WebSocketRoute
from starlette.staticfiles import StaticFiles
from starlette.websockets import WebSocket

from config import settings
from db import Chat, Message, db_session
from util.signer import Signer
from ws_handler import command_router

cookie_signer = Signer(settings.webui.signing_key, settings.webui.auth_expiration)


class SignedCookieAuthenticationBackend(AuthenticationBackend):
    COOKIE_NAME = "session"

    async def authenticate(self, conn):
        if self.COOKIE_NAME not in conn.cookies:
            return

        token = conn.cookies[self.COOKIE_NAME]
        if cookie_signer.check_token(token):
            return AuthCredentials(["authenticated"]), SimpleUser("someone")


async def index(request: Request):
    if not request.user.is_authenticated:
        return RedirectResponse("/login")

    return FileResponse("webui/html/chat.html")


async def login_page(request: Request):
    if request.user.is_authenticated:
        return RedirectResponse("/")

    return FileResponse("webui/html/login.html")


async def login_attempt(request: Request):
    if request.user.is_authenticated:
        return RedirectResponse("/")

    async with request.form() as form:
        token = form.get("token", "")

        given = sha512(token.encode("utf-8")).hexdigest().encode("utf-8")
        expected = settings.webui.password_sha512.encode("utf-8")

        if not secrets.compare_digest(given, expected):
            return JSONResponse({}, status_code=401)

    session_cookie = SignedCookieAuthenticationBackend.COOKIE_NAME
    new_token = cookie_signer.get_signed_token()

    response = JSONResponse({}, status_code=200)
    response.set_cookie(session_cookie, new_token)

    return response


async def login(request: Request):
    if request.method == "GET":
        return await login_page(request)
    return await login_attempt(request)


async def ws(websocket: WebSocket):
    token = websocket.query_params.get("token", "")

    if not token or not cookie_signer.check_token(token):
        await websocket.close()

    await websocket.accept()

    async for message in websocket.iter_json():
        result = command_router.execute(message)
        await websocket.send_json(result)

    await websocket.close()


async def ws_test(request):
    return FileResponse("webui/html/ws_test.html")


app = Starlette(
    debug=settings.webui.debug,
    routes=[
        Route("/", index, methods=["GET"]),
        Route("/login", login, methods=["GET", "POST"]),
        Mount("/static", StaticFiles(directory="webui/static")),
        WebSocketRoute("/ws", endpoint=ws),
        Route("/ws_test", ws_test),
    ],
    middleware=[
        Middleware(
            AuthenticationMiddleware, backend=SignedCookieAuthenticationBackend()
        )
    ],
)
