import asyncio
import secrets
import typing as t
from asyncio.queues import Queue
from hashlib import sha512
from uuid import UUID, uuid4

from starlette.applications import Starlette
from starlette.authentication import AuthCredentials, AuthenticationBackend, SimpleUser
from starlette.middleware import Middleware
from starlette.middleware.authentication import AuthenticationMiddleware
from starlette.requests import Request
from starlette.responses import FileResponse, JSONResponse, RedirectResponse
from starlette.routing import Mount, Route, WebSocketRoute
from starlette.staticfiles import StaticFiles
from starlette.types import Receive, Scope, Send
from starlette.websockets import WebSocket
from starlette.exceptions import HTTPException

from config import settings
from util.signer import Signer
from ws_handler import command_router

cookie_signer = Signer(settings.webui.signing_key, settings.webui.auth_expiration)
ws_clients: t.Mapping[UUID, Queue] = {}


class SignedCookieAuthenticationBackend(AuthenticationBackend):
    COOKIE_NAME = "session"

    async def authenticate(self, conn):
        if self.COOKIE_NAME not in conn.cookies:
            return

        token = conn.cookies[self.COOKIE_NAME]
        if cookie_signer.check_token(token):
            return AuthCredentials(["authenticated"]), SimpleUser("someone")


class SecuredStatic(StaticFiles):
    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if not scope["user"].is_authenticated:
            raise HTTPException(404)

        return await super().__call__(scope, receive, send)


async def index(request: Request):
    if not request.user.is_authenticated:
        return RedirectResponse("/login")

    return FileResponse("webui/index.html")


async def login_page(request: Request):
    if request.user.is_authenticated:
        return RedirectResponse("/")

    return FileResponse("webui/login.html")


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


async def ws_url(request: Request):
    if not request.user.is_authenticated:
        return RedirectResponse("/login")

    return JSONResponse({"url": settings.webui.websocket_endpoint})


async def ws(websocket: WebSocket):
    token = websocket.query_params.get("token", "")

    if not token or not cookie_signer.check_token(token):
        await websocket.close()

    await websocket.accept()

    client_id = uuid4()
    sender_queue = Queue()
    ws_clients[client_id] = sender_queue

    async def receiver():
        async for message in websocket.iter_json():
            result = command_router.execute(message)
            await websocket.send_json(result)

    async def sender():
        try:
            while True:
                message = await sender_queue.get()
                await websocket.send_json(message)
                sender_queue.task_done()
        except asyncio.CancelledError:
            return

    sender_coroutine = sender()
    await asyncio.wait(
        (receiver(), sender_coroutine), return_when=asyncio.FIRST_COMPLETED
    )

    sender_coroutine.throw(asyncio.CancelledError)
    sender_coroutine.close()

    del ws_clients[client_id]
    await websocket.close()


app = Starlette(
    debug=settings.webui.debug,
    routes=[
        Route("/", index, methods=["GET"]),
        Route("/login", login, methods=["GET", "POST"]),
        Route("/ws_url", ws_url, methods=["GET"]),
        Mount("/static", StaticFiles(directory="webui/static")),
        Mount("/secured", SecuredStatic(directory="webui/secured")),
        WebSocketRoute("/ws", endpoint=ws),
    ],
    middleware=[
        Middleware(
            AuthenticationMiddleware, backend=SignedCookieAuthenticationBackend()
        )
    ],
)
