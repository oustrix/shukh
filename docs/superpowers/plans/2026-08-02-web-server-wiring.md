# Сшивка веб-клиента с сервером (Спец 3, итерация 3) — план

> **For agentic workers:** REQUIRED SUB-SKILL: используй superpowers:subagent-driven-development
> (рекомендуется) или superpowers:executing-plans для исполнения плана задача-за-задачей.
> Шаги размечены чекбоксами (`- [ ]`).

**Goal:** Соединить готовые Слои 0–2 с веб-клиентом так, чтобы партию с друзьями можно было
сыграть в браузере от входа в комнату до финиша, включая голосование R-8.6 и реконнект.

**Architecture:** Клиент остаётся зрителем пушей (W2-1) и не считает правила (W2-2). Новый слой
сети в `web/src`: `contract/wire.ts` (единственное место, знающее JSON), `net/rooms.ts` (HTTP),
`transport/ws.ts` (сокет + реконнект), `store/GameProvider.tsx` (стор на комнату). Сервер
получает семь аддитивных правок (§7 спека). Расхождение ручных зеркал (W-3) ловится
golden-фикстурами: Go-тест пишет реальные сообщения в `server/testdata/wire/*.json`, TS-тест их
декодирует.

**Tech Stack:** Go 1.22+ (`net/http` method+path patterns, `github.com/coder/websocket`), Vite +
React 19 + TypeScript (strict) + Vitest + @testing-library/react + zustand + motion. **Новых
зависимостей нет ни на одной стороне.**

**Спек:** [`docs/superpowers/specs/2026-08-02-web-server-wiring-design.md`](../specs/2026-08-02-web-server-wiring-design.md)

## Global Constraints

- **Гейт Go (из корня), обязателен в конце каждой Go-задачи:** `go build ./... && go test ./...`.
- **Гейт web (из `web/`), обязателен в конце каждой web-задачи:** `npm run typecheck && npm run lint && npm test`.
- **Клиент НЕ считает правила (W2-2).** Любая интерактивность выводится ТОЛЬКО из `snapshot.legal`
  через хелперы `isLegal`/`isCardPlayable`. Единственное исключение — `claimSubjective`: движок
  сознательно не кладёт его в `legal` (см. комментарий в `engine/legal.go:16`), это всегда
  доступная кнопка, валидируемая сервером на сабмите.
- **Ручной миррор (W-3).** `SeatView`, `Action`, `GameEvent` — зеркала `engine/*.go`; НЕ добавляй в
  них полей, которых нет в движке. Поля уровня комнаты (`stage`, `host`, `you`, `roomCode`,
  `voteDeadline`) живут на `GameSnapshot`, НЕ на `SeatView`.
- **Стабильный ключ карты (W2-4).** React-ключ/`layoutId`/выбор — по `cardKey(card)`, НИКОГДА по
  индексу массива.
- **Слои 0/1 правим только аддитивно.** Движок остаётся без сети, времени и RNG.
- **reduced-motion** обеспечен глобально в `web/src/main.tsx`; CSS-анимации обязаны иметь
  `@media (prefers-reduced-motion: reduce)`-выключение.
- **Коммиты** заканчиваются трейлером:
  `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`
- **Git-идентичность в этом репозитории не настроена.** Если `git commit` падает с «Author identity
  unknown», коммить как `git -c user.name=oustrix -c user.email=dsfomin9@gmail.com commit ...`
  (совпадает с автором всей истории).
- **Каждая задача — свежий сабагент**, свой цикл тестов, заканчивается зелёным гейтом.

---

## Структура файлов

**Go — правится (аддитивно):**
- `game/projection.go` — `Update.Host`; заполнение в `project()`.
- `server/protocol.go` — `ServerMsg.Host`; `"host"` в `encodeUpdate`.
- `server/conn.go` — `error{seatNotFound}` перед закрытием сокета.
- `server/http.go` — маршруты `/api/*` + `/ws/*`, кука `Path=/`, проба `/me`, CORS, `OriginPatterns`.
- `cmd/shukh-server/main.go` — флаги `-origins`, `-cross-site`.

**Go — создаётся:**
- `server/protocol_golden_test.go` + `server/testdata/wire/*.json` — golden-фикстуры протокола.

**Web — создаётся:**
- `web/src/contract/wire.ts` — кодек JSON ↔ типы (единственное место со знанием формата).
- `web/src/net/rooms.ts` — HTTP-клиент комнат (`createRoom`/`joinRoom`/`me`).
- `web/src/transport/ws.ts` — WS-транспорт: реконнект, `reqId`/ack/error, `ConnStatus`.
- `web/src/store/GameProvider.tsx` — контекст стора на комнату.
- `web/src/ui/screens/Room.tsx` — экран комнаты, ветвление по `stage` (W3-1).
- `web/src/ui/table/SeatMenu.tsx` — меню адресных действий по сопернику.
- `web/.env.development` — `VITE_API_ORIGIN` для дева.

**Web — правится:** `contract/types.ts`, `contract/transport.ts`, `store/game.ts`, `routes.ts`,
`App.tsx`, `ui/screens/Join.tsx`, `ui/screens/Lobby.tsx`, `ui/screens/Table.tsx`,
`ui/table/ActionBar.tsx`, `ui/table/OpponentSeat.tsx`, `ui/table/ShukhVoteModal.tsx`,
`ui/table/Table.module.css`.

**Web — удаляется:** `transport/scripted.ts`(+тест), `transport/mock.ts`(+тест),
`fixtures/scenario.ts`(+тест).

---

## Task 1: `host` в проекции (Слои 1–2)

**Files:**
- Modify: `game/projection.go` (структура `Update`, функция `project`)
- Modify: `server/protocol.go` (`ServerMsg`, `encodeUpdate`)
- Test: `game/projection_test.go`, `server/protocol_msg_test.go`

**Interfaces:**
- Produces: `game.Update.Host engine.SeatID` — место текущего хоста; в JSON поле `"host"` (число,
  всегда присутствует в `update`, включая место 0).

- [ ] **Step 1: Написать падающий тест в `game/projection_test.go`**

```go
func TestUpdateCarriesHostSeat(t *testing.T) {
	s := NewSession(Config{Rules: engine.RuleSet{DeckSize: engine.Deck36}, Mode: engine.Middle}, "p-host", "Host")
	if err := s.Join("p-bob", "Bob"); err != nil {
		t.Fatalf("join: %v", err)
	}
	up, err := s.SnapshotFor("p-bob")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if up.Host != 0 {
		t.Fatalf("host must be seat 0, got %d", up.Host)
	}
	// Хост ушёл из лобби → роль мигрирует на следующего (L2-3), и это видно в проекции.
	s.Leave("p-host")
	up, err = s.SnapshotFor("p-bob")
	if err != nil {
		t.Fatalf("snapshot after leave: %v", err)
	}
	if up.Host != 0 {
		t.Fatalf("after migration Bob (seat 0) must be host, got %d", up.Host)
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./game/ -run TestUpdateCarriesHostSeat -v`
Expected: FAIL — `up.Host undefined (type Update has no field or method Host)`.

- [ ] **Step 3: Добавить поле и заполнение в `game/projection.go`**

В структуру `Update` (после `Stage`):

```go
	// Host is the seat currently holding the host role (Lobby-only powers: SetConfig/
	// Start). Exposed so the client knows whether to offer them — including after a
	// host migration (L2-3), which the leaving client cannot observe otherwise.
	Host engine.SeatID
```

В `project()`, сразу после `seat, _ := s.seatOf(id)`:

```go
	hostSeat, _ := s.seatOf(s.host)
	up := Update{
		Stage:  s.stage,
		Host:   hostSeat,
		Roster: roster,
		Events: events,
	}
```

- [ ] **Step 4: Убедиться, что тест проходит**

Run: `go test ./game/ -run TestUpdateCarriesHostSeat -v`
Expected: PASS

- [ ] **Step 5: Написать падающий тест кодека в `server/protocol_msg_test.go`**

```go
func TestEncodeUpdateCarriesHost(t *testing.T) {
	up := game.Update{Stage: game.Lobby, Host: 2, Roster: []game.SeatMeta{{Seat: 0, Name: "A"}}}
	msg := encodeUpdate(0, "ABCD", up, nil)
	if msg.Host == nil {
		t.Fatal("update must carry host")
	}
	if *msg.Host != 2 {
		t.Fatalf("host = %d, want 2", *msg.Host)
	}
	// Место 0 — валидный хост и обязано попасть в JSON (указатель, не omitempty-число).
	zero := encodeUpdate(0, "ABCD", game.Update{Stage: game.Lobby, Host: 0}, nil)
	data, err := json.Marshal(zero)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"host":0`) {
		t.Fatalf("host 0 must be emitted, got %s", data)
	}
}
```

- [ ] **Step 6: Убедиться, что тест падает**

Run: `go test ./server/ -run TestEncodeUpdateCarriesHost -v`
Expected: FAIL — `msg.Host undefined`.

- [ ] **Step 7: Добавить поле в `ServerMsg` и его заполнение**

В `server/protocol.go`, в блок `// update` структуры `ServerMsg`, сразу после `You`:

```go
	Host         *int   `json:"host,omitempty"` // pointer: seat 0 is a valid host
```

В `encodeUpdate`, рядом с `yi := int(you)`:

```go
	hi := int(u.Host)
```

и в возвращаемую структуру, после `You: &yi`:

```go
		Host:         &hi,
```

- [ ] **Step 8: Убедиться, что тесты проходят, и прогнать гейт**

Run: `go build ./... && go test ./...`
Expected: всё зелёное.

- [ ] **Step 9: Коммит**

```bash
git add game/projection.go game/projection_test.go server/protocol.go server/protocol_msg_test.go
git commit -m "feat(game,server): host в проекции — клиент знает, кому показывать Старт (§7.1)"
```

---

## Task 2: `error{seatNotFound}` перед закрытием сокета

**Files:**
- Modify: `server/conn.go` (функция `serveConn`)
- Test: `server/conn_test.go`

**Interfaces:**
- Consumes: `errorMsg(reqID, code, message string) ServerMsg` из `server/protocol.go`.
- Produces: сокет, у которого `Subscribe` отказал, получает текстовый кадр
  `{"type":"error","code":"seatNotFound",...}` до закрытия.

- [ ] **Step 1: Написать падающий тест в `server/conn_test.go`**

```go
func TestConnectAfterSeatReleasedSendsError(t *testing.T) {
	h := NewHub(NewMemStore(), newFakeClock(time.Unix(0, 0)))
	code, tok, room := h.CreateRoom(game.Config{Rules: engine.RuleSet{DeckSize: engine.Deck36}, Mode: engine.Middle}, "Host")
	srv := httptest.NewServer(NewServer(h).Handler())
	defer srv.Close()

	pid, ok := room.playerFor(tok)
	if !ok {
		t.Fatal("token must resolve")
	}
	room.session.Leave(pid) // grace истёк в лобби: место освобождено, токен ещё жив

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/r/"+code,
		&websocket.DialOptions{HTTPHeader: http.Header{"Cookie": []string{cookieName(code) + "=" + string(tok)}}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("сокет закрылся молча — клиенту нечего показать: %v", err)
	}
	var msg ServerMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != "error" || msg.Code != "seatNotFound" {
		t.Fatalf("got %+v, want error/seatNotFound", msg)
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./server/ -run TestConnectAfterSeatReleasedSendsError -v`
Expected: FAIL — чтение возвращает ошибку закрытия, тест сообщает «сокет закрылся молча».

- [ ] **Step 3: Отправить причину перед возвратом (`server/conn.go`)**

В `serveConn` заменить блок отказа `Subscribe`:

```go
	ch, unsub, err := r.session.Subscribe(pid)
	if err != nil {
		// The seat is gone (grace expired in the lobby). The write pump is not running
		// yet, so write the reason inline: closing silently leaves the browser unable to
		// tell "seat lost" from "server down", and it would retry forever (§7.2).
		if data, mErr := json.Marshal(errorMsg("", "seatNotFound", err.Error())); mErr == nil {
			wctx, wcancel := context.WithTimeout(ctx, writeTimeout)
			_ = c.Write(wctx, websocket.MessageText, data)
			wcancel()
		}
		return
	}
```

- [ ] **Step 4: Убедиться, что тест проходит, и прогнать гейт**

Run: `go test ./server/ -run TestConnectAfterSeatReleasedSendsError -v && go build ./... && go test ./...`
Expected: PASS, гейт зелёный.

- [ ] **Step 5: Коммит**

```bash
git add server/conn.go server/conn_test.go
git commit -m "fix(server): причина seatNotFound уходит в сокет до закрытия (§7.2)"
```

---

## Task 3: переезд маршрутов на `/api` + `/ws`, кука `Path=/`, режим cross-site

**Files:**
- Modify: `server/http.go` (`NewServer`, `Handler`, `roomCookie`)
- Modify: `cmd/shukh-server/main.go`
- Test: `server/http_test.go` (обновить пути), `server/integration_test.go` (обновить пути)

**Interfaces:**
- Produces:
  - `server.Options{Origins []string, CrossSite bool}`
  - `server.NewServer(hub *Hub, opts Options) *Server` — **сигнатура меняется**, все вызовы
    обновляются в этой задаче.
  - Маршруты: `POST /api/rooms`, `POST /api/rooms/{code}/join`, `GET /ws/{code}`.
  - Кука: `Path=/`, `SameSite=Lax` (дефолт) либо `SameSite=None; Secure` при `CrossSite`.

- [ ] **Step 1: Написать падающий тест в `server/http_test.go`**

```go
func TestRoutesAndCookieScope(t *testing.T) {
	h := NewHub(NewMemStore(), newFakeClock(time.Unix(0, 0)))
	srv := httptest.NewServer(NewServer(h, Options{}).Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/rooms", "application/json", strings.NewReader(`{"name":"Host"}`))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer resp.Body.Close()
	var created struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	if created.Code == "" {
		t.Fatal("create must return a code")
	}
	ck := findCookie(resp.Cookies(), cookieName(created.Code))
	if ck == nil {
		t.Fatal("create must Set-Cookie")
	}
	// Кука обязана уходить и на /api/..., и на /ws/... — значит Path=/ (§7.4).
	if ck.Path != "/" {
		t.Fatalf("cookie Path = %q, want \"/\"", ck.Path)
	}
	if ck.SameSite != http.SameSiteLaxMode || ck.Secure {
		t.Fatalf("default cookie must be Lax and non-Secure, got SameSite=%v Secure=%v", ck.SameSite, ck.Secure)
	}
}

func TestCrossSiteCookieMode(t *testing.T) {
	h := NewHub(NewMemStore(), newFakeClock(time.Unix(0, 0)))
	srv := httptest.NewServer(NewServer(h, Options{CrossSite: true}).Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/rooms", "application/json", strings.NewReader(`{"name":"Host"}`))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer resp.Body.Close()
	var created struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	ck := findCookie(resp.Cookies(), cookieName(created.Code))
	if ck == nil {
		t.Fatal("create must Set-Cookie")
	}
	if ck.SameSite != http.SameSiteNoneMode || !ck.Secure {
		t.Fatalf("cross-site cookie must be None+Secure, got SameSite=%v Secure=%v", ck.SameSite, ck.Secure)
	}
}
```

- [ ] **Step 2: Убедиться, что тесты падают**

Run: `go test ./server/ -run 'TestRoutesAndCookieScope|TestCrossSiteCookieMode' -v`
Expected: FAIL — `NewServer` принимает один аргумент; `/api/rooms` даёт 404.

- [ ] **Step 3: Переписать `server/http.go`**

```go
// Options configures the transport-facing policy of the HTTP layer: which browser
// origins may talk to it (CORS + WS origin check) and whether the reconnect cookie
// must survive a cross-site deployment (W3-4).
type Options struct {
	Origins   []string // allowed browser origins, e.g. "http://localhost:5173"; empty = same-origin only
	CrossSite bool     // true → SameSite=None; Secure (requires TLS)
}

type Server struct {
	hub  *Hub
	opts Options
}

func NewServer(hub *Hub, opts Options) *Server { return &Server{hub: hub, opts: opts} }

// Handler builds the router. Go 1.22 method+path patterns; {code} via PathValue.
// /api/* and /ws/* are the server's namespace; /r/CODE is deliberately left free so
// the SPA can own the invite link (W3-2, D-2).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/rooms", s.createRoom)
	mux.HandleFunc("POST /api/rooms/{code}/join", s.joinRoom)
	mux.HandleFunc("GET /ws/{code}", s.connect)
	return mux
}

// cookieName scopes one cookie per room so several rooms coexist in one browser.
func cookieName(code string) string { return "shukh_" + code }

// roomCookie builds the reconnect cookie. Path is "/" because the token must reach
// both /api/rooms/{code}/... and /ws/{code} (§7.4); the per-room name keeps rooms
// isolated from each other.
func (s *Server) roomCookie(code string, tok Token) *http.Cookie {
	c := &http.Cookie{
		Name:     cookieName(code),
		Value:    string(tok),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if s.opts.CrossSite {
		c.SameSite = http.SameSiteNoneMode
		c.Secure = true
	}
	return c
}
```

В `createRoom` и `joinRoom` заменить `http.SetCookie(w, roomCookie(code, tok))` на
`http.SetCookie(w, s.roomCookie(code, tok))`.

- [ ] **Step 4: Обновить точку запуска `cmd/shukh-server/main.go`**

```go
	handler := server.NewServer(hub, server.Options{}).Handler()
```

- [ ] **Step 5: Обновить пути в существующих тестах**

В `server/http_test.go` и `server/integration_test.go` заменить:
`"/r"` → `"/api/rooms"`, `"/r/"+code+"/join"` → `"/api/rooms/"+code+"/join"`,
`"/r/"+code` (для WS-дозвона) → `"/ws/"+code`, и все `NewServer(h)` → `NewServer(h, Options{})`.

Найти все места: `grep -rn '"/r' server/ cmd/` и `grep -rn 'NewServer(' server/ cmd/`.

- [ ] **Step 6: Прогнать гейт**

Run: `go build ./... && go test ./...`
Expected: всё зелёное.

- [ ] **Step 7: Коммит**

```bash
git add server/http.go server/http_test.go server/integration_test.go cmd/shukh-server/main.go
git commit -m "feat(server): API на /api + /ws, кука Path=/ и режим cross-site (§7.3/§7.4)"
```

---

## Task 4: проба места `GET /api/rooms/{code}/me`

**Files:**
- Modify: `server/http.go` (`Handler`, новый хендлер `me`)
- Test: `server/http_test.go`

**Interfaces:**
- Consumes: `Options`, `s.roomCookie` из Task 3.
- Produces: `GET /api/rooms/{code}/me` → `200 {"seat":N}` | `401 {"error":"seatNotFound"}` |
  `404 {"error":"roomNotFound"}`.

- [ ] **Step 1: Написать падающий тест в `server/http_test.go`**

```go
func TestProbeMe(t *testing.T) {
	h := NewHub(NewMemStore(), newFakeClock(time.Unix(0, 0)))
	srv := httptest.NewServer(NewServer(h, Options{}).Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/rooms", "application/json", strings.NewReader(`{"name":"Host"}`))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var created struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	ck := findCookie(resp.Cookies(), cookieName(created.Code))

	probe := func(code string, cookie *http.Cookie) (int, string) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/rooms/"+code+"/me", nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("probe: %v", err)
		}
		defer r.Body.Close()
		var body struct {
			Seat  *int   `json:"seat"`
			Error string `json:"error"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Seat != nil {
			return r.StatusCode, "seat"
		}
		return r.StatusCode, body.Error
	}

	if st, what := probe(created.Code, ck); st != http.StatusOK || what != "seat" {
		t.Fatalf("with cookie: %d/%s, want 200/seat", st, what)
	}
	if st, what := probe(created.Code, nil); st != http.StatusUnauthorized || what != "seatNotFound" {
		t.Fatalf("without cookie: %d/%s, want 401/seatNotFound", st, what)
	}
	if st, what := probe("ZZZZ", ck); st != http.StatusNotFound || what != "roomNotFound" {
		t.Fatalf("unknown room: %d/%s, want 404/roomNotFound", st, what)
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./server/ -run TestProbeMe -v`
Expected: FAIL — маршрут не зарегистрирован, все ответы 404 без тела.

- [ ] **Step 3: Добавить маршрут и хендлер в `server/http.go`**

В `Handler()` после строки с `join`:

```go
	mux.HandleFunc("GET /api/rooms/{code}/me", s.me)
```

Новый хендлер (рядом с `joinRoom`):

```go
// me reports whether the caller already holds a seat in this room. The browser WS
// API hides the handshake status, so a failed socket is indistinguishable from a
// dead server without this probe; it also drives the invite-link flow (§7.7).
func (s *Server) me(w http.ResponseWriter, req *http.Request) {
	code := req.PathValue("code")
	room, ok := s.hub.Room(code)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "roomNotFound"})
		return
	}
	ck, err := req.Cookie(cookieName(code))
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "seatNotFound"})
		return
	}
	pid, ok := room.playerFor(Token(ck.Value))
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "seatNotFound"})
		return
	}
	seat := room.seatOf(pid)
	if seat < 0 {
		// Token is valid but the seat was released (grace expired in the lobby).
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "seatNotFound"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"seat": int(seat)})
}
```

- [ ] **Step 4: Убедиться, что тест проходит, и прогнать гейт**

Run: `go test ./server/ -run TestProbeMe -v && go build ./... && go test ./...`
Expected: PASS, гейт зелёный.

- [ ] **Step 5: Коммит**

```bash
git add server/http.go server/http_test.go
git commit -m "feat(server): проба места GET /api/rooms/{code}/me (§7.7)"
```

---

## Task 5: CORS и `OriginPatterns` для WS

**Files:**
- Modify: `server/http.go` (`Handler`, `connect`, новый метод `cors`)
- Modify: `cmd/shukh-server/main.go` (флаги `-origins`, `-cross-site`)
- Test: `server/http_test.go`

**Interfaces:**
- Consumes: `Options.Origins` из Task 3.
- Produces: обёртка CORS вокруг всего роутера; `websocket.Accept` с `OriginPatterns`;
  флаги `-origins` (список через запятую) и `-cross-site`.

- [ ] **Step 1: Написать падающий тест в `server/http_test.go`**

```go
func TestCORSAllowlist(t *testing.T) {
	h := NewHub(NewMemStore(), newFakeClock(time.Unix(0, 0)))
	srv := httptest.NewServer(NewServer(h, Options{Origins: []string{"http://localhost:5173"}}).Handler())
	defer srv.Close()

	// preflight от разрешённого origin
	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/api/rooms", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Allow-Origin = %q, want the echoed origin (\"*\" is illegal with credentials)", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Allow-Credentials = %q, want true", got)
	}

	// чужой origin не получает разрешения
	req2, _ := http.NewRequest(http.MethodOptions, srv.URL+"/api/rooms", nil)
	req2.Header.Set("Origin", "http://evil.example")
	req2.Header.Set("Access-Control-Request-Method", "POST")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("preflight2: %v", err)
	}
	resp2.Body.Close()
	if got := resp2.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Allow-Origin for foreign origin = %q, want empty", got)
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./server/ -run TestCORSAllowlist -v`
Expected: FAIL — preflight отдаёт 405, заголовков нет.

- [ ] **Step 3: Добавить CORS в `server/http.go`**

Обернуть роутер и добавить методы:

```go
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/rooms", s.createRoom)
	mux.HandleFunc("POST /api/rooms/{code}/join", s.joinRoom)
	mux.HandleFunc("GET /api/rooms/{code}/me", s.me)
	mux.HandleFunc("GET /ws/{code}", s.connect)
	return s.cors(mux)
}

// cors applies the allowlist to browser requests. The origin is echoed rather than
// "*": the reconnect cookie makes every request credentialed, and the wildcard is
// illegal with credentials (§7.5).
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		origin := req.Header.Get("Origin")
		if origin != "" && s.originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}
		if req.Method == http.MethodOptions && req.Header.Get("Access-Control-Request-Method") != "" {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, req)
	})
}

func (s *Server) originAllowed(origin string) bool {
	return slices.Contains(s.opts.Origins, origin)
}

// originPatterns converts the allowlist into host patterns for the WS Origin check.
// Empty list → nil, and coder/websocket falls back to same-origin only.
func (s *Server) originPatterns() []string {
	out := make([]string, 0, len(s.opts.Origins))
	for _, o := range s.opts.Origins {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			out = append(out, u.Host)
		}
	}
	return out
}
```

Импорты: добавить `"net/url"` и `"slices"`.

- [ ] **Step 4: Заменить `InsecureSkipVerify` на `OriginPatterns` в `connect`**

```go
	c, err := websocket.Accept(w, req, &websocket.AcceptOptions{OriginPatterns: s.originPatterns()})
```

Удалить комментарий `TODO(prod): InsecureSkipVerify ...` целиком — задача закрыта.

- [ ] **Step 5: Добавить флаги в `cmd/shukh-server/main.go`**

```go
	addr := flag.String("addr", ":8080", "listen address")
	origins := flag.String("origins", "", "comma-separated browser origins allowed to call the API and open sockets, e.g. http://localhost:5173")
	crossSite := flag.Bool("cross-site", false, "issue the reconnect cookie as SameSite=None; Secure (needed only when the SPA is on a different site; requires TLS)")
	flag.Parse()

	var allowed []string
	if *origins != "" {
		allowed = strings.Split(*origins, ",")
	}

	hub := server.NewHub(server.NewMemStore(), server.NewRealClock())
	hub.StartSweeper()

	handler := server.NewServer(hub, server.Options{Origins: allowed, CrossSite: *crossSite}).Handler()
```

Импорт: добавить `"strings"`.

- [ ] **Step 6: Прогнать гейт**

Run: `go build ./... && go test ./...`
Expected: всё зелёное. Существующие WS-тесты дозваниваются без заголовка `Origin`
(не браузер), поэтому проверка происхождения их не задевает.

- [ ] **Step 7: Коммит**

```bash
git add server/http.go server/http_test.go cmd/shukh-server/main.go
git commit -m "feat(server): CORS-allowlist + OriginPatterns для WS — снят TODO(prod) CSWSH (§7.5/§7.6)"
```

---

## Task 6: golden-фикстуры протокола

**Files:**
- Create: `server/protocol_golden_test.go`
- Create (генерируется тестом): `server/testdata/wire/*.json`

**Interfaces:**
- Produces: файлы `server/testdata/wire/<name>.json` — по одному сообщению `update` в каждом,
  отформатированные `json.MarshalIndent(msg, "", "  ")`. Их читает TS-тест из Task 8.
- Набор фикстур (имена файлов фиксированы, TS-тест на них опирается):
  `lobby.json`, `playing.json`, `vote_open.json`, `all_actions.json`, `all_events.json`.

- [ ] **Step 1: Написать тест-генератор `server/protocol_golden_test.go`**

Фикстуры собираются из **вручную построенных** `game.Update` — цель теста в том, чтобы
зафиксировать форму провода, а не гонять движок; так фикстуры детерминированы и покрывают все 12
действий и 17 событий, включая редкие.

```go
package server

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/oustrix/shukh/engine"
	"github.com/oustrix/shukh/game"
)

var updateGolden = flag.Bool("update", false, "rewrite server/testdata/wire/*.json")

// Golden fixtures pin the exact wire shape of every message the browser can receive.
// The TS mirror (web/src/contract/wire.test.ts) decodes these very files, so a change
// on either side that the other does not follow fails a test instead of a game (W3-3).
func TestProtocolGolden(t *testing.T) {
	c := func(rank int, suit engine.Suit) engine.Card { return engine.Card{Rank: engine.Rank(rank), Suit: suit} }
	view := &engine.SeatView{
		Rules:        engine.RuleSet{DeckSize: engine.Deck36},
		Mode:         engine.Middle,
		Phase:        engine.Playing,
		You:          1,
		Turn:         1,
		Hand:         []engine.Card{c(6, engine.Hearts), c(14, engine.Spades)},
		ShukhPending: 1,
		Opponents:    []engine.OpponentView{{Seat: 0, HandCount: 5, ShukhPending: 0, Live: true}},
		Table:        []engine.TableCard{{Card: c(7, engine.Clubs), By: 0}},
		Discard:      3,
		Talon:        18,
		Live:         map[engine.SeatID]bool{0: true, 1: true},
		Finish:       []engine.SeatID{},
	}
	voteView := *view
	voteView.Vote = &engine.VoteView{Claimant: 0, Target: 1, Code: engine.Sh6, Voted: []engine.SeatID{0}}

	roster := []game.SeatMeta{{Seat: 0, Name: "Вера"}, {Seat: 1, Name: "Боря"}}
	deadline := int64(1754130000000)

	cases := []struct {
		name string
		msg  ServerMsg
	}{
		{"lobby", encodeUpdate(1, "ABCD", game.Update{Stage: game.Lobby, Host: 0, Roster: roster}, nil)},
		{"playing", encodeUpdate(1, "ABCD", game.Update{
			Stage: game.Playing, Host: 0, Roster: roster, View: view,
			Legal:  []engine.Action{engine.PlayCard{Card: c(14, engine.Spades)}, engine.TakeBottomAndPass{}},
			Events: []engine.Event{engine.CardPlayed{Seat: 0, Card: c(7, engine.Clubs)}},
		}, nil)},
		{"vote_open", encodeUpdate(1, "ABCD", game.Update{
			Stage: game.Playing, Host: 0, Roster: roster, View: &voteView,
			Legal:  []engine.Action{engine.Vote{Voter: 1, Support: true}, engine.Vote{Voter: 1, Support: false}},
			Events: []engine.Event{engine.VoteOpened{Claimant: 0, Target: 1, Code: engine.Sh6}},
		}, &deadline)},
		{"all_actions", encodeUpdate(1, "ABCD", game.Update{
			Stage: game.Playing, Host: 0, Roster: roster, View: view,
			Legal: []engine.Action{
				engine.PlayCard{Card: c(6, engine.Hearts)},
				engine.TakeBottomAndPass{},
				engine.PodkladkaWest{},
				engine.DiscardWest{},
				engine.ClaimShukh{Target: 0, Code: engine.Sh2},
				engine.GiveShukhCard{Card: c(14, engine.Spades)},
				engine.TakeShukhCards{Seat: 1},
				engine.DeclareOneCard{Seat: 1},
				engine.AskCount{Target: 0},
				engine.AskAboutWest{Target: 0},
				engine.ClaimSubjective{Claimant: 1, Target: 0, Code: engine.Sh9},
				engine.Vote{Voter: 1, Support: false},
			},
		}, nil)},
		{"all_events", encodeUpdate(1, "ABCD", game.Update{
			Stage: game.Playing, Host: 0, Roster: roster, View: view,
			Events: []engine.Event{
				engine.GameStarted{Turn: 0},
				engine.CardPlayed{Seat: 0, Card: c(7, engine.Clubs)},
				engine.ConClosed{By: 0},
				engine.ConSwept{Cards: []engine.Card{c(7, engine.Clubs)}},
				engine.PlayerFinished{Seat: 0, Place: 1},
				engine.GameFinished{Finish: []engine.SeatID{0, 1}},
				engine.CardsTaken{Seat: 1, Cards: []engine.Card{c(7, engine.Clubs)}},
				engine.PodkladkaPlayed{Seat: 1, Eater: 0},
				engine.TurnSkipped{Seat: 1},
				engine.ShukhAssessed{Offender: 1, Code: engine.Sh11},
				engine.ActionReverted{Seat: 1},
				engine.ShukhPaid{Offender: 1, From: 0, Card: c(7, engine.Clubs)},
				engine.ShukhCardsTaken{Seat: 1, Cards: []engine.Card{c(7, engine.Clubs)}},
				engine.OneCardDeclared{Seat: 1},
				engine.WestDiscarded{Seat: 1},
				engine.VoteOpened{Claimant: 0, Target: 1, Code: engine.Sh6},
				engine.VoteResolved{Code: engine.Sh8, Overturned: true},
			},
		}, nil)},
	}

	dir := filepath.Join("testdata", "wire")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.MarshalIndent(tc.msg, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got = append(got, '\n')
			path := filepath.Join(dir, tc.name+".json")
			if *updateGolden {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatalf("write: %v", err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s (run `go test ./server/ -update` to create): %v", path, err)
			}
			if string(got) != string(want) {
				t.Fatalf("wire shape changed for %s.\n--- got ---\n%s\n--- want ---\n%s", tc.name, got, want)
			}
		})
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает без фикстур**

Run: `go test ./server/ -run TestProtocolGolden -v`
Expected: FAIL — «read testdata/wire/lobby.json ... no such file».

- [ ] **Step 3: Сгенерировать фикстуры**

Run: `go test ./server/ -run TestProtocolGolden -update`
Затем убедиться глазами, что `server/testdata/wire/all_events.json` содержит все 17 типов событий,
а `all_actions.json` — все 12 типов действий, и ни одного `"type": "unknown"` (он означал бы, что
в `encodeAction`/`encodeEvent` не хватает ветки).

Run: `grep -c '"type"' server/testdata/wire/all_events.json` (ожидаемо ≥ 17)

- [ ] **Step 4: Убедиться, что тест проходит, и прогнать гейт**

Run: `go test ./server/ -run TestProtocolGolden -v && go build ./... && go test ./...`
Expected: PASS, гейт зелёный.

- [ ] **Step 5: Коммит**

```bash
git add server/protocol_golden_test.go server/testdata/wire
git commit -m "test(server): golden-фикстуры протокола — общий контракт с TS-зеркалом (W3-3)"
```

---

## Task 7: контракт TS — дописать зеркала до полноты

**Files:**
- Modify: `web/src/contract/types.ts`
- Modify: `web/src/contract/transport.ts`
- Test: `web/src/contract/types.test.ts`

**Interfaces:**
- Produces (используется всеми последующими web-задачами):
  - `Stage = 'lobby' | 'playing' | 'finished'`
  - `ShukhCode = 2 | 3 | 6 | 8 | 9 | 10 | 11 | 12`; `SUBJECTIVE_CODES: readonly [6, 9, 10]`
  - `VoteView { claimant, target, code, voted }`; `SeatView.vote?: VoteView`
  - `SeatMeta { seat, name }` (без `ready`)
  - `GameSnapshot { roomCode, you, stage, host, seats, view, legal, voteDeadline? }`
    (без `shukhVote`)
  - `Action` — 12 членов; `GameEvent` — 17 членов
  - `ConnState`, `ProtocolError`, `ConnStatus`, `TransportHandlers`, `Transport`

- [ ] **Step 1: Написать падающий тест в `web/src/contract/types.test.ts`**

Дописать к существующим тестам:

```ts
import { actionsEqual, isLegal, SUBJECTIVE_CODES } from './types'
import type { Action } from './types'

describe('расширенный union действий', () => {
  it('различает адресные действия по цели', () => {
    const a: Action = { type: 'askCount', target: 1 }
    const b: Action = { type: 'askCount', target: 2 }
    expect(actionsEqual(a, a)).toBe(true)
    expect(actionsEqual(a, b)).toBe(false)
  })

  it('различает субъективный ШУХ по цели и коду', () => {
    const a: Action = { type: 'claimSubjective', claimant: 0, target: 1, code: 6 }
    const b: Action = { type: 'claimSubjective', claimant: 0, target: 1, code: 9 }
    expect(actionsEqual(a, b)).toBe(false)
  })

  it('различает голоса за и против', () => {
    const forShukh: Action = { type: 'vote', vote: 'forShukh' }
    const against: Action = { type: 'vote', vote: 'againstShukh' }
    expect(actionsEqual(forShukh, against)).toBe(false)
    expect(isLegal([against], forShukh)).toBe(false)
    expect(isLegal([forShukh, against], against)).toBe(true)
  })

  it('субъективные коды — ровно Ш-6/Ш-9/Ш-10 (R-8.4/R-8.7/R-8.8)', () => {
    expect([...SUBJECTIVE_CODES]).toEqual([6, 9, 10])
  })
})
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `cd web && npx vitest run src/contract/types.test.ts`
Expected: FAIL — `SUBJECTIVE_CODES` не экспортируется, типы `askCount`/`vote` не входят в `Action`.

- [ ] **Step 3: Дописать `web/src/contract/types.ts`**

Заменить `ShukhCode`, `SeatMeta`, `GameSnapshot`, `Action`, `GameEvent` и удалить `ShukhVote`:

```ts
// Коды ШУХов (§7 правил): объективные ловит движок, субъективные идут через голосование R-8.6.
export type ShukhCode = 2 | 3 | 6 | 8 | 9 | 10 | 11 | 12

// Ш-6 «завис» (R-8.4), Ш-9 «зря крикнул» (R-8.7), Ш-10 «небрежность» (R-8.8) —
// единственные, что предъявляются вручную через claimSubjective.
export const SUBJECTIVE_CODES = [6, 9, 10] as const satisfies readonly ShukhCode[]

// Публичная сводка открытого разбора R-8.6 (зеркало engine.VoteView, §8.3 Слоя 2).
// voted — ФАКТ голосования, без содержания: бюллетень тайный до резолва (§8.4).
export interface VoteView {
  claimant: SeatID
  target: SeatID
  code: ShukhCode
  voted: SeatID[]
}

export type Stage = 'lobby' | 'playing' | 'finished'

export interface SeatMeta {
  seat: SeatID
  name: string
}

export interface GameSnapshot {
  roomCode: string
  you: SeatID // своё место известно и в лобби, где view === null
  stage: Stage
  host: SeatID // чьи Старт/настройки (мигрирует, L2-3)
  seats: SeatMeta[]
  view: SeatView | null
  legal: Action[]
  voteDeadline?: number // unix-мс, пока идёт разбор R-8.6
}

export type Action =
  | { type: 'playCard'; card: Card }
  | { type: 'takeBottomAndPass' }
  | { type: 'podkladkaWest' }
  | { type: 'discardWest' }
  | { type: 'claimShukh'; target: SeatID; code: ShukhCode }
  | { type: 'giveShukhCard'; card: Card }
  | { type: 'takeShukhCards'; seat: SeatID }
  | { type: 'declareOneCard'; seat: SeatID }
  | { type: 'askCount'; target: SeatID }
  | { type: 'askAboutWest'; target: SeatID }
  | { type: 'claimSubjective'; claimant: SeatID; target: SeatID; code: ShukhCode }
  | { type: 'vote'; vote: 'forShukh' | 'againstShukh' }
```

В `SeatView` добавить поле:

```ts
  vote?: VoteView // открытый разбор R-8.6; отсутствует, когда разбора нет
```

В `GameEvent` дописать четыре члена:

```ts
  | { type: 'oneCardDeclared'; seat: SeatID }
  | { type: 'westDiscarded'; seat: SeatID }
  | { type: 'voteOpened'; claimant: SeatID; target: SeatID; code: ShukhCode }
  | { type: 'voteResolved'; code: ShukhCode; overturned: boolean }
```

В `actionKey` дописать ветки перед `default`:

```ts
    case 'askCount':
    case 'askAboutWest':
      return `${a.type}:${a.target}`
    case 'declareOneCard':
      return `declareOneCard:${a.seat}`
    case 'claimSubjective':
      return `claimSubjective:${a.target}:${a.code}`
    case 'vote':
      return `vote:${a.vote}`
```

Удалить интерфейс `ShukhVote` целиком.

- [ ] **Step 4: Переписать шов транспорта `web/src/contract/transport.ts`**

```ts
import type { Action, GameEvent, GameSnapshot } from './types'

// Состояние соединения (§8 спека). lost — терминальное: место или комната потеряны,
// повторять подключение бессмысленно.
export type ConnState = 'connecting' | 'open' | 'reconnecting' | 'lost'

// Ошибка протокола: code — стабильный код Слоя 2 (§10 дизайна Слоя 2), message — текст.
export interface ProtocolError {
  code: string
  message: string
}

export interface ConnStatus {
  state: ConnState
  error?: ProtocolError
}

export interface TransportHandlers {
  onSnapshot: (s: GameSnapshot) => void
  onEvent: (e: GameEvent) => void
  onStatus: (s: ConnStatus) => void
}

// Шов между UI и сетью (W-2). Единственная реализация — transport/ws.ts; тесты
// подставляют инлайновый двойник.
export interface Transport {
  subscribe(h: TransportHandlers): () => void
  send(action: Action): void
  close(): void
}
```

- [ ] **Step 5: Убедиться, что новые тесты проходят**

Run: `cd web && npx vitest run src/contract/types.test.ts`
Expected: PASS. Остальные файлы пока не компилируются (используют удалённые типы) — их чинят
следующие задачи; это ожидаемо, гейт прогоняем в Step 6 после починки прямых потребителей.

- [ ] **Step 6: Починить компиляцию потребителей контракта**

`grep -rln 'ShukhVote\|selectShukhVote\|ready' web/src` — во всех найденных местах убрать
обращения к удалённым полям. Конкретно:

- `web/src/fixtures/game.ts` — снять `ready: true` с трёх мест и добавить обязательные поля
  снапшота:

```ts
export const gameSnapshot: GameSnapshot = {
  roomCode: 'DEMO',
  you: 0,
  stage: 'playing',
  host: 0,
  seats: [
    { seat: 0, name: 'Аня' },
    { seat: 1, name: 'Боря' },
    { seat: 2, name: 'Вера' },
  ],
  // ...остальное без изменений
```

- `web/src/ui/screens/Lobby.tsx` — убрать вывод `s.ready` (полноценно лобби переписывается в Task 13).
- `web/src/ui/screens/Table.tsx` и `web/src/store/game.ts` — временно убрать `selectShukhVote` и
  рендер `<ShukhVoteModal>`; модалка переписывается на реальные данные в Task 16.
- `web/src/ui/table/ShukhVoteModal.tsx` и его тест — временно исключить из сборки нельзя, поэтому
  на этом шаге допустимо оставить их нетронутыми только если `typecheck` зелёный; иначе
  закомментировать тело компонента заглушкой `export function ShukhVoteModal() { return null }`
  и удалить тесты, ссылающиеся на `ShukhVote` (Task 16 напишет их заново).

Run: `cd web && npm run typecheck && npm run lint && npm test`
Expected: всё зелёное.

- [ ] **Step 7: Коммит**

```bash
git add web/src/contract web/src/store web/src/ui
git commit -m "feat(web): контракт дописан до протокола — 12 действий, 17 событий, VoteView, stage/host/you (§5)"
```

---

## Task 8: кодек `wire.ts` на golden-фикстурах

**Files:**
- Create: `web/src/contract/wire.ts`
- Test: `web/src/contract/wire.test.ts`

**Interfaces:**
- Consumes: типы из Task 7; фикстуры `server/testdata/wire/*.json` из Task 6.
- Produces:
  - `decodeServerMsg(raw: unknown): Decoded`, где
    `Decoded = { kind: 'update'; snapshot: GameSnapshot; events: GameEvent[] } | { kind: 'ack'; reqId: string } | { kind: 'error'; reqId?: string; error: ProtocolError }`
  - `encodeAction(a: Action): unknown`
  - `class WireError extends Error`

- [ ] **Step 1: Написать падающий тест `web/src/contract/wire.test.ts`**

```ts
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { decodeServerMsg, encodeAction, WireError } from './wire'
import type { Action } from './types'

// __dirname в ESM-модулях Vitest не определён — путь берём от import.meta.url.
const wireDir = fileURLToPath(new URL('../../../server/testdata/wire/', import.meta.url))
const fixture = (name: string) => JSON.parse(readFileSync(`${wireDir}${name}.json`, 'utf8'))

describe('decodeServerMsg на golden-фикстурах сервера (W3-3)', () => {
  it('лобби: view отсутствует, места и хост на месте', () => {
    const d = decodeServerMsg(fixture('lobby'))
    expect(d.kind).toBe('update')
    if (d.kind !== 'update') return
    expect(d.snapshot.stage).toBe('lobby')
    expect(d.snapshot.view).toBeNull()
    expect(d.snapshot.you).toBe(1)
    expect(d.snapshot.host).toBe(0)
    expect(d.snapshot.seats).toEqual([
      { seat: 0, name: 'Вера' },
      { seat: 1, name: 'Боря' },
    ])
  })

  it('партия: рука, кон и легальные ходы декодируются', () => {
    const d = decodeServerMsg(fixture('playing'))
    if (d.kind !== 'update') throw new Error('want update')
    expect(d.snapshot.stage).toBe('playing')
    expect(d.snapshot.view?.hand).toHaveLength(2)
    expect(d.snapshot.view?.table[0]).toEqual({ card: { rank: 7, suit: '♣' }, by: 0 })
    expect(d.snapshot.legal).toContainEqual({ type: 'takeBottomAndPass' })
    expect(d.events[0]).toEqual({ type: 'cardPlayed', seat: 0, card: { rank: 7, suit: '♣' } })
  })

  it('открытый разбор: VoteView и дедлайн доезжают', () => {
    const d = decodeServerMsg(fixture('vote_open'))
    if (d.kind !== 'update') throw new Error('want update')
    expect(d.snapshot.view?.vote).toEqual({ claimant: 0, target: 1, code: 6, voted: [0] })
    expect(d.snapshot.voteDeadline).toBe(1754130000000)
  })

  it('все 12 действий сервера декодируются без потерь', () => {
    const d = decodeServerMsg(fixture('all_actions'))
    if (d.kind !== 'update') throw new Error('want update')
    expect(d.snapshot.legal).toHaveLength(12)
    expect(new Set(d.snapshot.legal.map((a) => a.type)).size).toBe(12)
  })

  it('все 17 событий сервера декодируются без потерь', () => {
    const d = decodeServerMsg(fixture('all_events'))
    if (d.kind !== 'update') throw new Error('want update')
    expect(d.events).toHaveLength(17)
    expect(new Set(d.events.map((e) => e.type)).size).toBe(17)
  })
})

describe('decodeServerMsg — конверты и защита', () => {
  it('ack и error', () => {
    expect(decodeServerMsg({ type: 'ack', reqId: 'r1' })).toEqual({ kind: 'ack', reqId: 'r1' })
    expect(decodeServerMsg({ type: 'error', reqId: 'r2', code: 'notYours', message: 'nope' })).toEqual({
      kind: 'error',
      reqId: 'r2',
      error: { code: 'notYours', message: 'nope' },
    })
  })

  it('неизвестный тип события — громкая ошибка, а не тихий пропуск (§5)', () => {
    const bad = { ...fixture('playing'), events: [{ type: 'somethingNew', seat: 0 }] }
    expect(() => decodeServerMsg(bad)).toThrow(WireError)
  })

  it('неизвестный конверт — тоже ошибка', () => {
    expect(() => decodeServerMsg({ type: 'gossip' })).toThrow(WireError)
  })
})

describe('encodeAction', () => {
  it('голос кодируется строкой, понятной серверу', () => {
    const a: Action = { type: 'vote', vote: 'againstShukh' }
    expect(encodeAction(a)).toEqual({ type: 'vote', vote: 'againstShukh' })
  })
})
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `cd web && npx vitest run src/contract/wire.test.ts`
Expected: FAIL — модуль `./wire` не найден.

- [ ] **Step 3: Написать `web/src/contract/wire.ts`**

```ts
import type {
  Action,
  Card,
  GameEvent,
  GameSnapshot,
  SeatID,
  SeatMeta,
  SeatView,
  ShukhCode,
  Stage,
  VoteView,
} from './types'
import type { ProtocolError } from './transport'

// Расхождение ручных зеркал (W-3) — единственная поломка, которую мы обязаны заметить
// немедленно, поэтому кодек не «прощает» неизвестное, а падает.
export class WireError extends Error {
  constructor(message: string) {
    super(`wire: ${message}`)
    this.name = 'WireError'
  }
}

export type Decoded =
  | { kind: 'update'; snapshot: GameSnapshot; events: GameEvent[] }
  | { kind: 'ack'; reqId: string }
  | { kind: 'error'; reqId?: string; error: ProtocolError }

type Obj = Record<string, unknown>

function obj(v: unknown, what: string): Obj {
  if (typeof v !== 'object' || v === null || Array.isArray(v)) throw new WireError(`${what} must be an object`)
  return v as Obj
}
function num(v: unknown, what: string): number {
  if (typeof v !== 'number' || Number.isNaN(v)) throw new WireError(`${what} must be a number`)
  return v
}
function str(v: unknown, what: string): string {
  if (typeof v !== 'string') throw new WireError(`${what} must be a string`)
  return v
}
function bool(v: unknown, what: string): boolean {
  if (typeof v !== 'boolean') throw new WireError(`${what} must be a boolean`)
  return v
}
function arr(v: unknown, what: string): unknown[] {
  if (!Array.isArray(v)) throw new WireError(`${what} must be an array`)
  return v
}

function decodeCard(v: unknown): Card {
  const o = obj(v, 'card')
  const suit = str(o.suit, 'card.suit')
  if (suit !== '♠' && suit !== '♥' && suit !== '♦' && suit !== '♣') throw new WireError(`unknown suit ${suit}`)
  return { suit, rank: num(o.rank, 'card.rank') }
}
const decodeCards = (v: unknown): Card[] => arr(v, 'cards').map(decodeCard)
const decodeSeats = (v: unknown): SeatID[] => arr(v, 'seats').map((s) => num(s, 'seat'))
const decodeCode = (v: unknown): ShukhCode => num(v, 'code') as ShukhCode

function decodeVote(v: unknown): VoteView {
  const o = obj(v, 'vote')
  return {
    claimant: num(o.claimant, 'vote.claimant'),
    target: num(o.target, 'vote.target'),
    code: decodeCode(o.code),
    voted: decodeSeats(o.voted),
  }
}

function decodeView(v: unknown): SeatView {
  const o = obj(v, 'view')
  const rules = obj(o.rules, 'view.rules')
  const mode = str(o.mode, 'view.mode')
  if (mode !== 'guard' && mode !== 'middle' && mode !== 'culture') throw new WireError(`unknown mode ${mode}`)
  const phase = str(o.phase, 'view.phase')
  if (phase !== 'playing' && phase !== 'finished') throw new WireError(`unknown phase ${phase}`)
  const deckSize = num(rules.deckSize, 'rules.deckSize')
  if (deckSize !== 36 && deckSize !== 52) throw new WireError(`unknown deck size ${deckSize}`)
  const live: Record<number, boolean> = {}
  for (const [k, val] of Object.entries(obj(o.live, 'view.live'))) live[Number(k)] = bool(val, 'view.live[]')
  const view: SeatView = {
    rules: {
      deckSize,
      podkladkaSnizu: bool(rules.podkladkaSnizu, 'rules.podkladkaSnizu'),
      jokers: bool(rules.jokers, 'rules.jokers'),
    },
    mode,
    phase,
    you: num(o.you, 'view.you'),
    turn: num(o.turn, 'view.turn'),
    hand: decodeCards(o.hand),
    shukhPending: num(o.shukhPending, 'view.shukhPending'),
    opponents: arr(o.opponents, 'view.opponents').map((x) => {
      const p = obj(x, 'opponent')
      return {
        seat: num(p.seat, 'opponent.seat'),
        handCount: num(p.handCount, 'opponent.handCount'),
        shukhPending: num(p.shukhPending, 'opponent.shukhPending'),
        live: bool(p.live, 'opponent.live'),
      }
    }),
    table: arr(o.table, 'view.table').map((x) => {
      const t = obj(x, 'tableCard')
      return { card: decodeCard(t.card), by: num(t.by, 'tableCard.by') }
    }),
    discard: num(o.discard, 'view.discard'),
    talon: num(o.talon, 'view.talon'),
    live,
    finish: decodeSeats(o.finish),
  }
  if (o.vote !== undefined) view.vote = decodeVote(o.vote)
  return view
}

function decodeAction(v: unknown): Action {
  const o = obj(v, 'action')
  const type = str(o.type, 'action.type')
  switch (type) {
    case 'playCard':
      return { type, card: decodeCard(o.card) }
    case 'takeBottomAndPass':
    case 'podkladkaWest':
    case 'discardWest':
      return { type }
    case 'claimShukh':
      return { type, target: num(o.target, 'target'), code: decodeCode(o.code) }
    case 'giveShukhCard':
      return { type, card: decodeCard(o.card) }
    case 'takeShukhCards':
      return { type, seat: num(o.seat, 'seat') }
    case 'declareOneCard':
      return { type, seat: num(o.seat, 'seat') }
    case 'askCount':
    case 'askAboutWest':
      return { type, target: num(o.target, 'target') }
    case 'claimSubjective':
      return {
        type,
        claimant: num(o.claimant, 'claimant'),
        target: num(o.target, 'target'),
        code: decodeCode(o.code),
      }
    case 'vote': {
      const vote = str(o.vote, 'vote')
      if (vote !== 'forShukh' && vote !== 'againstShukh') throw new WireError(`unknown vote ${vote}`)
      return { type, vote }
    }
    default:
      throw new WireError(`unknown action type ${type} — зеркала разошлись с engine/action.go`)
  }
}

function decodeEvent(v: unknown): GameEvent {
  const o = obj(v, 'event')
  const type = str(o.type, 'event.type')
  switch (type) {
    case 'gameStarted':
      return { type, turn: num(o.turn, 'turn') }
    case 'cardPlayed':
      return { type, seat: num(o.seat, 'seat'), card: decodeCard(o.card) }
    case 'conClosed':
      return { type, by: num(o.by, 'by') }
    case 'conSwept':
      return { type, cards: decodeCards(o.cards) }
    case 'playerFinished':
      return { type, seat: num(o.seat, 'seat'), place: num(o.place, 'place') }
    case 'gameFinished':
      return { type, finish: decodeSeats(o.finish) }
    case 'cardsTaken':
      return { type, seat: num(o.seat, 'seat'), cards: decodeCards(o.cards) }
    case 'podkladkaPlayed':
      return { type, seat: num(o.seat, 'seat'), eater: num(o.eater, 'eater') }
    case 'turnSkipped':
    case 'actionReverted':
    case 'oneCardDeclared':
    case 'westDiscarded':
      return { type, seat: num(o.seat, 'seat') }
    case 'shukhAssessed':
      return { type, offender: num(o.offender, 'offender'), code: decodeCode(o.code) }
    case 'shukhPaid':
      return {
        type,
        offender: num(o.offender, 'offender'),
        from: num(o.from, 'from'),
        card: decodeCard(o.card),
      }
    case 'shukhCardsTaken':
      return { type, seat: num(o.seat, 'seat'), cards: decodeCards(o.cards) }
    case 'voteOpened':
      return {
        type,
        claimant: num(o.claimant, 'claimant'),
        target: num(o.target, 'target'),
        code: decodeCode(o.code),
      }
    case 'voteResolved':
      return { type, code: decodeCode(o.code), overturned: bool(o.overturned, 'overturned') }
    default:
      throw new WireError(`unknown event type ${type} — зеркала разошлись с engine/event.go`)
  }
}

function decodeStage(v: unknown): Stage {
  const s = str(v, 'stage')
  if (s !== 'lobby' && s !== 'playing' && s !== 'finished') throw new WireError(`unknown stage ${s}`)
  return s
}

function decodeRoster(v: unknown): SeatMeta[] {
  if (v === undefined) return []
  return arr(v, 'roster').map((x) => {
    const m = obj(x, 'seatMeta')
    return { seat: num(m.seat, 'seat'), name: str(m.name, 'name') }
  })
}

export function decodeServerMsg(raw: unknown): Decoded {
  const o = obj(raw, 'message')
  switch (str(o.type, 'type')) {
    case 'update': {
      const snapshot: GameSnapshot = {
        roomCode: str(o.roomCode, 'roomCode'),
        you: num(o.you, 'you'),
        stage: decodeStage(o.stage),
        host: num(o.host, 'host'),
        seats: decodeRoster(o.roster),
        view: o.view === undefined || o.view === null ? null : decodeView(o.view),
        legal: o.legal === undefined ? [] : arr(o.legal, 'legal').map(decodeAction),
      }
      if (o.voteDeadline !== undefined) snapshot.voteDeadline = num(o.voteDeadline, 'voteDeadline')
      const events = o.events === undefined ? [] : arr(o.events, 'events').map(decodeEvent)
      return { kind: 'update', snapshot, events }
    }
    case 'ack':
      return { kind: 'ack', reqId: str(o.reqId, 'reqId') }
    case 'error':
      return {
        kind: 'error',
        reqId: o.reqId === undefined ? undefined : str(o.reqId, 'reqId'),
        error: { code: str(o.code, 'code'), message: o.message === undefined ? '' : str(o.message, 'message') },
      }
    default:
      throw new WireError(`unknown envelope type ${String(o.type)}`)
  }
}

// Форма действия на проводе совпадает с TS-типом один-в-один (см. decodeAction в
// server/protocol.go), поэтому кодирование тождественно. Функция существует как явный
// шов: если формы разойдутся, правка будет здесь, а не по всему UI.
export function encodeAction(a: Action): unknown {
  return a
}
```

- [ ] **Step 4: Убедиться, что тесты проходят**

Run: `cd web && npx vitest run src/contract/wire.test.ts`
Expected: PASS (все 8 тестов).

- [ ] **Step 5: Прогнать гейт и закоммитить**

Run: `cd web && npm run typecheck && npm run lint && npm test`

```bash
git add web/src/contract/wire.ts web/src/contract/wire.test.ts
git commit -m "feat(web): кодек wire.ts + тест на golden-фикстурах сервера (W3-3)"
```

---

## Task 9: HTTP-клиент комнат `net/rooms.ts`

**Files:**
- Create: `web/src/net/rooms.ts`
- Create: `web/src/net/rooms.test.ts`
- Create: `web/.env.development`

**Interfaces:**
- Produces:
  - `apiOrigin(): string` — база API (`import.meta.env.VITE_API_ORIGIN` либо
    `window.location.origin`)
  - `createRoom(name: string, config?: RoomConfig): Promise<{ code: string }>`
  - `joinRoom(code: string, name: string): Promise<{ seat: number }>`
  - `me(code: string): Promise<MeResult>`, где
    `MeResult = { kind: 'seat'; seat: number } | { kind: 'seatNotFound' } | { kind: 'roomNotFound' }`
  - `RoomConfig = { deckSize: 36 | 52; mode: 'guard' | 'middle' | 'culture' }`
  - `class ApiError extends Error { code: 'full' | 'duplicate' | 'roomNotFound' | 'unknown' }`

- [ ] **Step 1: Создать `web/.env.development`**

```
VITE_API_ORIGIN=http://localhost:8080
```

- [ ] **Step 2: Написать падающий тест `web/src/net/rooms.test.ts`**

```ts
import { createRoom, joinRoom, me, ApiError } from './rooms'

function mockFetch(status: number, body: unknown) {
  const spy = vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response)
  vi.stubGlobal('fetch', spy)
  return spy
}

afterEach(() => vi.unstubAllGlobals())

describe('net/rooms', () => {
  it('createRoom шлёт имя и конфиг и ВСЕГДА с credentials (кука комнаты)', async () => {
    const spy = mockFetch(200, { code: 'ABCD' })
    const res = await createRoom('Вера', { deckSize: 36, mode: 'middle' })
    expect(res).toEqual({ code: 'ABCD' })
    const [url, init] = spy.mock.calls[0]
    expect(String(url)).toMatch(/\/api\/rooms$/)
    expect(init.method).toBe('POST')
    expect(init.credentials).toBe('include')
    expect(JSON.parse(init.body)).toEqual({ name: 'Вера', config: { deckSize: 36, mode: 'middle' } })
  })

  it('joinRoom возвращает место', async () => {
    mockFetch(200, { seat: 2 })
    await expect(joinRoom('ABCD', 'Боря')).resolves.toEqual({ seat: 2 })
  })

  it('joinRoom → 409 превращается в типизированную ошибку', async () => {
    mockFetch(409, { error: 'full' })
    await expect(joinRoom('ABCD', 'Боря')).rejects.toMatchObject({ code: 'full' })
    await expect(joinRoom('ABCD', 'Боря')).rejects.toBeInstanceOf(ApiError)
  })

  it('me различает три исхода пробы', async () => {
    mockFetch(200, { seat: 1 })
    await expect(me('ABCD')).resolves.toEqual({ kind: 'seat', seat: 1 })
    mockFetch(401, { error: 'seatNotFound' })
    await expect(me('ABCD')).resolves.toEqual({ kind: 'seatNotFound' })
    mockFetch(404, { error: 'roomNotFound' })
    await expect(me('ABCD')).resolves.toEqual({ kind: 'roomNotFound' })
  })
})
```

- [ ] **Step 3: Убедиться, что тест падает**

Run: `cd web && npx vitest run src/net/rooms.test.ts`
Expected: FAIL — модуль `./rooms` не найден.

- [ ] **Step 4: Написать `web/src/net/rooms.ts`**

```ts
// HTTP-часть входа в комнату. Токен места живёт в HttpOnly-куке (L2-6), поэтому
// каждый запрос идёт с credentials:'include' — иначе кука не уедет на другой origin.

export interface RoomConfig {
  deckSize: 36 | 52
  mode: 'guard' | 'middle' | 'culture'
}

export type MeResult = { kind: 'seat'; seat: number } | { kind: 'seatNotFound' } | { kind: 'roomNotFound' }

export type ApiErrorCode = 'full' | 'duplicate' | 'roomNotFound' | 'unknown'

export class ApiError extends Error {
  readonly code: ApiErrorCode
  constructor(code: ApiErrorCode, message: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
  }
}

// Сервер живёт на отдельном origin (W3-4): в деве адрес приходит из .env.development,
// в проде — из переменной сборки; иначе считаем, что API на том же хосте.
export function apiOrigin(): string {
  const fromEnv = import.meta.env.VITE_API_ORIGIN
  return typeof fromEnv === 'string' && fromEnv !== '' ? fromEnv : window.location.origin
}

async function postJSON(path: string, body: unknown): Promise<Response> {
  return fetch(`${apiOrigin()}${path}`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

function errorCode(payload: unknown): ApiErrorCode {
  const raw = typeof payload === 'object' && payload !== null ? (payload as { error?: unknown }).error : undefined
  return raw === 'full' || raw === 'duplicate' || raw === 'roomNotFound' ? raw : 'unknown'
}

export async function createRoom(name: string, config?: RoomConfig): Promise<{ code: string }> {
  const resp = await postJSON('/api/rooms', config ? { name, config } : { name })
  const payload = await resp.json().catch(() => ({}))
  if (!resp.ok) throw new ApiError(errorCode(payload), 'не удалось создать комнату')
  return { code: String((payload as { code?: unknown }).code ?? '') }
}

export async function joinRoom(code: string, name: string): Promise<{ seat: number }> {
  const resp = await postJSON(`/api/rooms/${encodeURIComponent(code)}/join`, { name })
  const payload = await resp.json().catch(() => ({}))
  if (!resp.ok) throw new ApiError(errorCode(payload), 'не удалось занять место')
  return { seat: Number((payload as { seat?: unknown }).seat ?? 0) }
}

// Проба места: браузерный WebSocket не показывает статус неудавшегося рукопожатия,
// поэтому «нет места» и «сервер лежит» различаем этим запросом (§7.7).
export async function me(code: string): Promise<MeResult> {
  const resp = await fetch(`${apiOrigin()}/api/rooms/${encodeURIComponent(code)}/me`, {
    credentials: 'include',
  })
  if (resp.status === 404) return { kind: 'roomNotFound' }
  if (!resp.ok) return { kind: 'seatNotFound' }
  const payload = await resp.json().catch(() => ({}))
  return { kind: 'seat', seat: Number((payload as { seat?: unknown }).seat ?? 0) }
}
```

- [ ] **Step 5: Убедиться, что тесты проходят, прогнать гейт, закоммитить**

Run: `cd web && npx vitest run src/net/rooms.test.ts && npm run typecheck && npm run lint && npm test`

```bash
git add web/src/net web/.env.development
git commit -m "feat(web): HTTP-клиент комнат — create/join/me с credentials (§4)"
```

---

## Task 10: WS-транспорт с реконнектом

**Files:**
- Create: `web/src/transport/ws.ts`
- Create: `web/src/transport/ws.test.ts`

**Interfaces:**
- Consumes: `decodeServerMsg`/`encodeAction`/`WireError` (Task 8), `apiOrigin` (Task 9),
  `me` (Task 9), `Transport`/`ConnStatus` (Task 7).
- Produces:
  - `createWsTransport(code: string, deps?: WsDeps): Transport`
  - `interface WsDeps { socketFactory?: (url: string) => WsLike; schedule?: (fn: () => void, ms: number) => () => void; probe?: (code: string) => Promise<MeResult>; }`
  - `interface WsLike { send(data: string): void; close(): void; onopen: ((e?: unknown) => void) | null; onmessage: ((e: { data: string }) => void) | null; onclose: ((e?: unknown) => void) | null; onerror: ((e?: unknown) => void) | null; }`
  - Бэкофф: `500 * 2^n` мс, потолок `8000`.

- [ ] **Step 1: Написать падающий тест `web/src/transport/ws.test.ts`**

```ts
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { createWsTransport } from './ws'
import type { WsLike } from './ws'
import type { ConnStatus, TransportHandlers } from '../contract/transport'
import type { GameSnapshot } from '../contract/types'
import type { MeResult } from '../net/rooms'

const playingJSON = readFileSync(
  fileURLToPath(new URL('../../../server/testdata/wire/playing.json', import.meta.url)),
  'utf8',
)

class FakeSocket implements WsLike {
  static last: FakeSocket | null = null
  static created = 0
  sent: string[] = []
  closed = false
  onopen: ((e?: unknown) => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null
  onclose: ((e?: unknown) => void) | null = null
  onerror: ((e?: unknown) => void) | null = null
  constructor(readonly url: string) {
    FakeSocket.last = this
    FakeSocket.created += 1
  }
  send(data: string) {
    this.sent.push(data)
  }
  close() {
    this.closed = true
  }
  open() {
    this.onopen?.()
  }
  message(raw: string) {
    this.onmessage?.({ data: raw })
  }
  drop() {
    this.onclose?.()
  }
}

interface Harness {
  statuses: ConnStatus[]
  snapshots: GameSnapshot[]
  transport: ReturnType<typeof createWsTransport>
  runTimers: () => void
}

function harness(probeResult: 'seat' | 'seatNotFound' | 'roomNotFound' = 'seat'): Harness {
  FakeSocket.last = null
  FakeSocket.created = 0
  const pending: (() => void)[] = []
  const statuses: ConnStatus[] = []
  const snapshots: GameSnapshot[] = []
  const transport = createWsTransport('ABCD', {
    socketFactory: (url) => new FakeSocket(url),
    schedule: (fn) => {
      pending.push(fn)
      return () => {}
    },
    probe: async (): Promise<MeResult> =>
      probeResult === 'seat' ? { kind: 'seat', seat: 1 } : { kind: probeResult },
  })
  const handlers: TransportHandlers = {
    onSnapshot: (s) => snapshots.push(s),
    onEvent: () => {},
    onStatus: (s) => statuses.push(s),
  }
  transport.subscribe(handlers)
  return { statuses, snapshots, transport, runTimers: () => pending.splice(0).forEach((fn) => fn()) }
}

describe('transport/ws', () => {
  it('открытие сокета переводит в open, update отдаёт снапшот', () => {
    const h = harness()
    expect(h.statuses.at(-1)?.state).toBe('connecting')
    FakeSocket.last!.open()
    expect(h.statuses.at(-1)?.state).toBe('open')
    FakeSocket.last!.message(playingJSON)
    expect(h.snapshots).toHaveLength(1)
    expect(h.snapshots[0].stage).toBe('playing')
  })

  it('send уходит на провод с reqId; ack не меняет снапшот', () => {
    const h = harness()
    FakeSocket.last!.open()
    h.transport.send({ type: 'takeBottomAndPass' })
    const sent = JSON.parse(FakeSocket.last!.sent[0])
    expect(sent.type).toBe('action')
    expect(sent.action).toEqual({ type: 'takeBottomAndPass' })
    expect(typeof sent.reqId).toBe('string')
    FakeSocket.last!.message(JSON.stringify({ type: 'ack', reqId: sent.reqId }))
    expect(h.snapshots).toHaveLength(0)
  })

  it('error кладётся в статус, не роняя соединение', () => {
    const h = harness()
    FakeSocket.last!.open()
    FakeSocket.last!.message(JSON.stringify({ type: 'error', code: 'notYours', message: 'не твой ход' }))
    expect(h.statuses.at(-1)).toEqual({ state: 'open', error: { code: 'notYours', message: 'не твой ход' } })
  })

  it('обрыв ведёт в reconnecting и пересоздаёт сокет по таймеру', () => {
    const h = harness()
    FakeSocket.last!.open()
    FakeSocket.last!.drop()
    expect(h.statuses.at(-1)?.state).toBe('reconnecting')
    expect(FakeSocket.created).toBe(1)
    h.runTimers()
    expect(FakeSocket.created).toBe(2)
  })

  it('в reconnecting отправка отбрасывается (W3-5)', () => {
    const h = harness()
    FakeSocket.last!.open()
    const socket = FakeSocket.last!
    socket.drop()
    h.transport.send({ type: 'takeBottomAndPass' })
    expect(socket.sent).toHaveLength(0)
  })

  it('error{seatNotFound} по сокету — терминальный lost, реконнекта нет', () => {
    const h = harness()
    FakeSocket.last!.open()
    FakeSocket.last!.message(JSON.stringify({ type: 'error', code: 'seatNotFound', message: 'gone' }))
    expect(h.statuses.at(-1)?.state).toBe('lost')
    h.runTimers()
    expect(FakeSocket.created).toBe(1)
  })

  it('close() закрывает сокет и прекращает реконнект', () => {
    const h = harness()
    FakeSocket.last!.open()
    h.transport.close()
    expect(FakeSocket.last!.closed).toBe(true)
    h.runTimers()
    expect(FakeSocket.created).toBe(1)
  })
})
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `cd web && npx vitest run src/transport/ws.test.ts`
Expected: FAIL — модуль `./ws` не найден.

- [ ] **Step 3: Написать `web/src/transport/ws.ts`**

```ts
import type { Action } from '../contract/types'
import type { ConnState, ConnStatus, ProtocolError, Transport, TransportHandlers } from '../contract/transport'
import { decodeServerMsg, encodeAction, WireError } from '../contract/wire'
import { apiOrigin, me, type MeResult } from '../net/rooms'

// Минимальная часть WebSocket, которой пользуется транспорт: тесты подставляют двойник.
export interface WsLike {
  send(data: string): void
  close(): void
  onopen: ((e?: unknown) => void) | null
  onmessage: ((e: { data: string }) => void) | null
  onclose: ((e?: unknown) => void) | null
  onerror: ((e?: unknown) => void) | null
}

export interface WsDeps {
  socketFactory?: (url: string) => WsLike
  schedule?: (fn: () => void, ms: number) => () => void
  probe?: (code: string) => Promise<MeResult>
}

const BACKOFF_BASE_MS = 500
const BACKOFF_CAP_MS = 8000

// Терминальные причины: место или комната потеряны — повторять бессмысленно (§8).
const TERMINAL = new Set(['seatNotFound', 'roomNotFound'])

function wsURL(code: string): string {
  return `${apiOrigin().replace(/^http/, 'ws')}/ws/${encodeURIComponent(code)}`
}

export function createWsTransport(code: string, deps: WsDeps = {}): Transport {
  const makeSocket = deps.socketFactory ?? ((url: string) => new WebSocket(url) as unknown as WsLike)
  const schedule =
    deps.schedule ??
    ((fn: () => void, ms: number) => {
      const id = setTimeout(fn, ms)
      return () => clearTimeout(id)
    })
  const probe = deps.probe ?? me

  let handlers: TransportHandlers | null = null
  let socket: WsLike | null = null
  let status: ConnStatus = { state: 'connecting' }
  let attempt = 0
  let cancelRetry: (() => void) | null = null
  let stopped = false
  let seq = 0

  function setStatus(state: ConnState, error?: ProtocolError) {
    status = error ? { state, error } : { state }
    handlers?.onStatus(status)
  }

  function retryLater() {
    if (stopped) return
    const delay = Math.min(BACKOFF_BASE_MS * 2 ** attempt, BACKOFF_CAP_MS)
    attempt += 1
    cancelRetry = schedule(() => {
      cancelRetry = null
      connect()
    }, delay)
  }

  // Сокет не открылся: браузер не отдаёт статус рукопожатия, поэтому причину узнаём
  // пробой — иначе бесконечный бэкофф в стену на потерянном месте (§7.7).
  function diagnoseFailure() {
    void probe(code).then((res) => {
      if (stopped) return
      if (res.kind === 'seatNotFound' || res.kind === 'roomNotFound') {
        setStatus('lost', { code: res.kind, message: 'место или комната недоступны' })
        return
      }
      retryLater()
    })
  }

  function connect() {
    if (stopped) return
    const s = makeSocket(wsURL(code))
    socket = s
    let opened = false
    s.onopen = () => {
      opened = true
      attempt = 0
      setStatus('open')
    }
    s.onmessage = (e) => {
      let decoded
      try {
        decoded = decodeServerMsg(JSON.parse(e.data))
      } catch (err) {
        // Расхождение зеркал (W-3) — не глотаем: пусть будет видно в консоли и в статусе.
        const message = err instanceof WireError ? err.message : String(err)
        setStatus(status.state, { code: 'wire', message })
        return
      }
      if (decoded.kind === 'update') {
        handlers?.onSnapshot(decoded.snapshot)
        decoded.events.forEach((ev) => handlers?.onEvent(ev))
        return
      }
      if (decoded.kind === 'error') {
        if (TERMINAL.has(decoded.error.code)) {
          stopped = true
          socket?.close()
          setStatus('lost', decoded.error)
          return
        }
        setStatus(status.state, decoded.error)
      }
      // ack ничего не меняет: рендер идёт только из update (L2-4).
    }
    s.onclose = () => {
      if (stopped) return
      socket = null
      setStatus('reconnecting')
      if (opened) retryLater()
      else diagnoseFailure()
    }
    s.onerror = () => {
      /* onclose придёт следом и разберётся */
    }
  }

  return {
    subscribe(h) {
      handlers = h
      h.onStatus(status)
      connect()
      return () => {
        handlers = null
      }
    },
    send(action: Action) {
      // Отложенная доставка запрещена (W3-5): за время обрыва позиция ушла вперёд.
      if (stopped || status.state !== 'open' || !socket) return
      seq += 1
      socket.send(JSON.stringify({ type: 'action', action: encodeAction(action), reqId: `a${seq}` }))
    },
    close() {
      stopped = true
      cancelRetry?.()
      cancelRetry = null
      socket?.close()
      socket = null
      handlers = null
    },
  }
}
```

- [ ] **Step 4: Убедиться, что тесты проходят, прогнать гейт, закоммитить**

Run: `cd web && npx vitest run src/transport/ws.test.ts && npm run typecheck && npm run lint && npm test`

```bash
git add web/src/transport/ws.ts web/src/transport/ws.test.ts
git commit -m "feat(web): WS-транспорт — реконнект с бэкоффом, reqId/ack/error, проба причины (§8)"
```

---

## Task 11: стор на комнату вместо синглтона

**Files:**
- Modify: `web/src/store/game.ts`
- Create: `web/src/store/GameProvider.tsx`
- Modify: `web/src/store/game.test.ts`

**Interfaces:**
- Consumes: `Transport`/`ConnStatus` (Task 7).
- Produces:
  - `createGameStore(transport: Transport)` → zustand-стор с полями
    `{ snapshot, events, conn: ConnState, lastError: ProtocolError | null, play(action) }`
  - `GameProvider({ code, children })` — создаёт стор на `code`, закрывает транспорт при размонтировании
  - `useGame<T>(selector: (s: GameState) => T): T`
  - Селекторы: `selectSeats`, `selectView`, `selectLegal`, `selectStage`, `selectYou`,
    `selectHost`, `selectVote`, `selectVoteDeadline`, `selectConn`, `selectLastError`
  - `EVENTS_CAP = 100` (без изменений)

- [ ] **Step 1: Переписать тест `web/src/store/game.test.ts`**

```ts
import { createGameStore, EVENTS_CAP } from './game'
import type { Transport, TransportHandlers } from '../contract/transport'
import type { Action, GameEvent, GameSnapshot } from '../contract/types'

// Инлайновый двойник транспорта: отдельного файла-двойника больше нет (W3-7).
function fakeTransport() {
  let handlers: TransportHandlers | null = null
  const sent: Action[] = []
  const transport: Transport = {
    subscribe(h) {
      handlers = h
      return () => {
        handlers = null
      }
    },
    send: (a) => sent.push(a),
    close: () => {},
  }
  return {
    transport,
    sent,
    push: (s: GameSnapshot) => handlers?.onSnapshot(s),
    event: (e: GameEvent) => handlers?.onEvent(e),
    status: (s: Parameters<TransportHandlers['onStatus']>[0]) => handlers?.onStatus(s),
  }
}

const snap: GameSnapshot = {
  roomCode: 'ABCD',
  you: 1,
  stage: 'lobby',
  host: 0,
  seats: [{ seat: 0, name: 'Вера' }],
  view: null,
  legal: [],
}

describe('store/game', () => {
  it('снапшот из транспорта попадает в стор', () => {
    const f = fakeTransport()
    const store = createGameStore(f.transport)
    f.push(snap)
    expect(store.getState().snapshot?.roomCode).toBe('ABCD')
  })

  it('буфер событий хранит ПОСЛЕДНИЕ EVENTS_CAP', () => {
    const f = fakeTransport()
    const store = createGameStore(f.transport)
    for (let i = 0; i < EVENTS_CAP + 5; i += 1) f.event({ type: 'turnSkipped', seat: i })
    const events = store.getState().events
    expect(events).toHaveLength(EVENTS_CAP)
    expect(events[events.length - 1]).toEqual({ type: 'turnSkipped', seat: EVENTS_CAP + 4 })
  })

  it('статус соединения и последняя ошибка видны в сторе', () => {
    const f = fakeTransport()
    const store = createGameStore(f.transport)
    f.status({ state: 'reconnecting' })
    expect(store.getState().conn).toBe('reconnecting')
    f.status({ state: 'open', error: { code: 'notYours', message: 'не твой ход' } })
    expect(store.getState().conn).toBe('open')
    expect(store.getState().lastError).toEqual({ code: 'notYours', message: 'не твой ход' })
  })

  it('play уходит в транспорт', () => {
    const f = fakeTransport()
    const store = createGameStore(f.transport)
    store.getState().play({ type: 'takeBottomAndPass' })
    expect(f.sent).toEqual([{ type: 'takeBottomAndPass' }])
  })
})
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `cd web && npx vitest run src/store/game.test.ts`
Expected: FAIL — `subscribe` принимает два колбэка, полей `conn`/`lastError` нет.

- [ ] **Step 3: Переписать `web/src/store/game.ts`**

```ts
import { create } from 'zustand'
import type { ConnState, ProtocolError, Transport } from '../contract/transport'
import type { Action, GameEvent, GameSnapshot } from '../contract/types'

export interface GameState {
  snapshot: GameSnapshot | null
  events: GameEvent[]
  conn: ConnState
  lastError: ProtocolError | null
  play: (action: Action) => void
}

// Предел лога событий — событий за партию много; держим только последние.
export const EVENTS_CAP = 100

export const selectSnapshot = (s: GameState) => s.snapshot
export const selectSeats = (s: GameState) => s.snapshot?.seats ?? []
export const selectView = (s: GameState) => s.snapshot?.view ?? null
export const selectLegal = (s: GameState) => s.snapshot?.legal ?? []
export const selectStage = (s: GameState) => s.snapshot?.stage ?? null
export const selectYou = (s: GameState) => s.snapshot?.you ?? null
export const selectHost = (s: GameState) => s.snapshot?.host ?? null
export const selectVote = (s: GameState) => s.snapshot?.view?.vote ?? null
export const selectVoteDeadline = (s: GameState) => s.snapshot?.voteDeadline ?? null
export const selectConn = (s: GameState) => s.conn
export const selectLastError = (s: GameState) => s.lastError
export const selectEvents = (s: GameState) => s.events

// Создаёт изолированный стор поверх переданного транспорта. Подписка — ПОСЛЕ создания
// стора: транспорт пушит в уже готовый setState.
export function createGameStore(transport: Transport) {
  const store = create<GameState>(() => ({
    snapshot: null,
    events: [],
    conn: 'connecting',
    lastError: null,
    play: (action) => transport.send(action),
  }))
  transport.subscribe({
    onSnapshot: (snapshot) => store.setState({ snapshot }),
    onEvent: (event) => store.setState((s) => ({ events: [...s.events, event].slice(-EVENTS_CAP) })),
    onStatus: (status) => store.setState({ conn: status.state, lastError: status.error ?? null }),
  })
  return store
}

export type GameStore = ReturnType<typeof createGameStore>
```

- [ ] **Step 4: Написать `web/src/store/GameProvider.tsx`**

```tsx
import { createContext, useContext, useEffect, useMemo, type ReactNode } from 'react'
import { useStore } from 'zustand'
import { createGameStore, type GameState, type GameStore } from './game'
import { createWsTransport } from '../transport/ws'

// Стор живёт ровно столько, сколько открыта комната: код известен только на маршруте,
// а транспорт держит сокет, который обязан закрыться при уходе (синглтона больше нет).
const GameContext = createContext<GameStore | null>(null)

export function GameProvider({ code, children }: { code: string; children: ReactNode }) {
  const { store, close } = useMemo(() => {
    const transport = createWsTransport(code)
    return { store: createGameStore(transport), close: () => transport.close() }
  }, [code])
  useEffect(() => close, [close])
  return <GameContext.Provider value={store}>{children}</GameContext.Provider>
}

export function useGame<T>(selector: (s: GameState) => T): T {
  const store = useContext(GameContext)
  if (!store) throw new Error('useGame вне GameProvider')
  return useStore(store, selector)
}
```

- [ ] **Step 5: Убедиться, что тесты стора проходят**

Run: `cd web && npx vitest run src/store/game.test.ts`
Expected: PASS (4 теста). Экраны пока используют `useGameStore` — их чинит Task 12.

- [ ] **Step 6: Коммит** (гейт прогоняем в Task 12, когда экраны перейдут на новый стор)

```bash
git add web/src/store
git commit -m "feat(web): стор на комнату вместо синглтона + состояние соединения"
```

---

## Task 12: экраны — Join, Room с пробой, ветвление по `stage`

**Files:**
- Modify: `web/src/routes.ts`, `web/src/App.tsx`, `web/src/ui/screens/Join.tsx`
- Create: `web/src/ui/screens/Room.tsx`
- Modify: `web/src/ui/screens/Screens.module.css` (стили баннера/формы)
- Test: `web/src/App.test.tsx`, создать `web/src/ui/screens/Room.test.tsx`

**Interfaces:**
- Consumes: `me`/`joinRoom`/`createRoom` (Task 9), `GameProvider`/`useGame` (Task 11).
- Produces:
  - `ROOM_ROUTE = '/r/:code'`, `roomPath(code)` → `/r/CODE`; `TABLE_ROUTE`/`tablePath` удалены
  - `Room` — экран комнаты: проба → форма имени → `GameProvider` → ветвление по `stage`
  - `STORED_NAME_KEY = 'shukh.name'`

- [ ] **Step 1: Написать падающий тест `web/src/ui/screens/Room.test.tsx`**

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { Room } from './Room'
import { ROOM_ROUTE } from '../../routes'
import { ApiError, joinRoom, me } from '../../net/rooms'

// ESM-экспорты не патчатся через vi.spyOn — модуль подменяется целиком (vi.mock),
// ApiError берём настоящий, чтобы проверять реальную ветку обработки ошибки.
vi.mock('../../net/rooms', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../net/rooms')>()
  return { ...actual, me: vi.fn(), joinRoom: vi.fn() }
})

function renderAt(code: string) {
  return render(
    <MemoryRouter initialEntries={[`/r/${code}`]}>
      <Routes>
        <Route path={ROOM_ROUTE} element={<Room />} />
      </Routes>
    </MemoryRouter>,
  )
}

afterEach(() => vi.resetAllMocks())

describe('экран комнаты', () => {
  it('несуществующая комната → понятный экран, а не бесконечная загрузка', async () => {
    vi.mocked(me).mockResolvedValue({ kind: 'roomNotFound' })
    renderAt('ZZZZ')
    expect(await screen.findByText(/Комната не найдена/i)).toBeInTheDocument()
  })

  it('переход по инвайт-ссылке без куки → форма имени прямо здесь (§4)', async () => {
    vi.mocked(me).mockResolvedValue({ kind: 'seatNotFound' })
    vi.mocked(joinRoom).mockResolvedValue({ seat: 1 })
    renderAt('ABCD')
    const input = await screen.findByLabelText('Имя')
    await userEvent.type(input, 'Боря')
    await userEvent.click(screen.getByRole('button', { name: /Занять место/i }))
    expect(joinRoom).toHaveBeenCalledWith('ABCD', 'Боря')
  })

  it('409 full показывает причину под формой', async () => {
    vi.mocked(me).mockResolvedValue({ kind: 'seatNotFound' })
    vi.mocked(joinRoom).mockRejectedValue(new ApiError('full', 'нет мест'))
    renderAt('ABCD')
    await userEvent.type(await screen.findByLabelText('Имя'), 'Боря')
    await userEvent.click(screen.getByRole('button', { name: /Занять место/i }))
    expect(await screen.findByText(/Комната заполнена/i)).toBeInTheDocument()
  })
})
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `cd web && npx vitest run src/ui/screens/Room.test.tsx`
Expected: FAIL — модуль `./Room` не найден.

- [ ] **Step 3: Обновить `web/src/routes.ts`**

```ts
// Единый источник форм URL. /r/CODE — та самая шаримая инвайт-ссылка (D-2, W3-2);
// стадия партии живёт в снапшоте, а не в адресе (W3-1), поэтому маршрут один.
export const ROOM_ROUTE = '/r/:code'
export const roomPath = (code: string) => `/r/${code}`
export const STORED_NAME_KEY = 'shukh.name'
```

- [ ] **Step 4: Написать `web/src/ui/screens/Room.tsx`**

```tsx
import { useCallback, useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { ApiError, joinRoom, me } from '../../net/rooms'
import { STORED_NAME_KEY } from '../../routes'
import { GameProvider, useGame } from '../../store/GameProvider'
import { selectConn, selectLastError, selectSeats, selectStage, selectView } from '../../store/game'
import { Button } from '../kit/Button'
import { Lobby } from './Lobby'
import { Table } from './Table'
import styles from './Screens.module.css'

type Probe = 'checking' | 'seated' | 'needsName' | 'roomNotFound'

const joinErrorText: Record<string, string> = {
  full: 'Комната заполнена',
  duplicate: 'Такое имя уже занято',
  roomNotFound: 'Комната не найдена',
  unknown: 'Не удалось занять место',
}

export function Room() {
  const { code = '' } = useParams()
  const [probe, setProbe] = useState<Probe>('checking')
  const [name, setName] = useState(() => localStorage.getItem(STORED_NAME_KEY) ?? '')
  const [joinError, setJoinError] = useState<string | null>(null)

  const check = useCallback(async () => {
    const res = await me(code)
    setProbe(res.kind === 'seat' ? 'seated' : res.kind === 'roomNotFound' ? 'roomNotFound' : 'needsName')
  }, [code])

  useEffect(() => {
    void check()
  }, [check])

  if (probe === 'checking') return <div className={styles.centered}>Проверяем место…</div>
  if (probe === 'roomNotFound') {
    return (
      <div className={styles.centered}>
        <h2>Комната не найдена</h2>
        <p>Код {code} никому не принадлежит — возможно, комната уже закрылась.</p>
      </div>
    )
  }
  if (probe === 'needsName') {
    return (
      <form
        className={styles.centered}
        onSubmit={(e) => {
          e.preventDefault()
          setJoinError(null)
          localStorage.setItem(STORED_NAME_KEY, name.trim())
          void joinRoom(code, name.trim())
            .then(() => setProbe('seated'))
            .catch((err) => setJoinError(joinErrorText[err instanceof ApiError ? err.code : 'unknown']))
        }}
      >
        <h2>Комната {code}</h2>
        <input aria-label="Имя" placeholder="Имя" value={name} onChange={(e) => setName(e.target.value)} />
        <Button type="submit" disabled={name.trim() === ''}>
          Занять место
        </Button>
        {joinError && <p role="alert">{joinError}</p>}
      </form>
    )
  }
  return (
    <GameProvider code={code}>
      <RoomBody />
    </GameProvider>
  )
}

// Стадия приходит с сервера и правит экраном (W3-1): нажатый хостом «Начать» переводит
// в стол у всех, а переподключение возвращает туда, где партия сейчас.
function RoomBody() {
  const stage = useGame(selectStage)
  const conn = useGame(selectConn)
  const lastError = useGame(selectLastError)

  if (conn === 'lost') {
    return (
      <div className={styles.centered}>
        <h2>Место потеряно</h2>
        <p>{lastError?.code === 'roomNotFound' ? 'Комната закрылась.' : 'Место освободилось, пока вас не было.'}</p>
        <Button onClick={() => window.location.reload()}>Войти заново</Button>
      </div>
    )
  }
  return (
    <>
      {conn !== 'open' && (
        <div className={styles.connBanner} role="status">
          {conn === 'connecting' ? 'Подключение…' : 'Связь потеряна, переподключаемся…'}
        </div>
      )}
      {stage === null && <div className={styles.centered}>Загрузка комнаты…</div>}
      {stage === 'lobby' && <Lobby />}
      {(stage === 'playing' || stage === 'finished') && <Table />}
      {stage === 'finished' && <FinishBanner />}
    </>
  )
}

// Итог партии (R-10.1): порядок выхода публичен, новая партия в этой итерации не
// запускается — серия R-10.2 отложена дорожной картой.
function FinishBanner() {
  const view = useGame(selectView)
  const seats = useGame(selectSeats)
  const nameOf = (seat: number) => seats.find((s) => s.seat === seat)?.name ?? `Игрок ${seat}`
  return (
    <div className={styles.finishBanner} role="status">
      <h3>Партия окончена</h3>
      <ol>
        {(view?.finish ?? []).map((seat) => (
          <li key={seat}>{nameOf(seat)}</li>
        ))}
      </ol>
      <Button onClick={() => window.location.assign('/')}>Выйти</Button>
    </div>
  )
}
```

- [ ] **Step 5: Упростить `web/src/ui/screens/Join.tsx`**

Экран входа теперь только создаёт комнату; вход по коду — навигация на `/r/CODE`, где проба
и форма имени живут в одном месте.

```tsx
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { createRoom } from '../../net/rooms'
import { roomPath, STORED_NAME_KEY } from '../../routes'
import { Button } from '../kit/Button'
import styles from './Screens.module.css'

export function Join() {
  const [name, setName] = useState(() => localStorage.getItem(STORED_NAME_KEY) ?? '')
  const [code, setCode] = useState('')
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()
  const trimmedName = name.trim()

  const remember = () => localStorage.setItem(STORED_NAME_KEY, trimmedName)

  return (
    <div className={styles.centered}>
      <h1>Шух</h1>
      <input aria-label="Имя" placeholder="Имя" value={name} onChange={(e) => setName(e.target.value)} />
      <Button
        disabled={trimmedName === ''}
        onClick={() => {
          setError(null)
          remember()
          void createRoom(trimmedName)
            .then((r) => navigate(roomPath(r.code)))
            .catch(() => setError('Не удалось создать комнату'))
        }}
      >
        Создать комнату
      </Button>
      <form
        onSubmit={(e) => {
          e.preventDefault()
          remember()
          navigate(roomPath(code.trim().toUpperCase()))
        }}
      >
        <input
          aria-label="Код комнаты"
          placeholder="Код комнаты"
          value={code}
          onChange={(e) => setCode(e.target.value)}
        />
        <Button type="submit" disabled={code.trim() === ''}>
          Войти по коду
        </Button>
      </form>
      {error && <p role="alert">{error}</p>}
    </div>
  )
}
```

- [ ] **Step 6: Обновить `web/src/App.tsx` и стили**

```tsx
import { Routes, Route } from 'react-router-dom'
import { ROOM_ROUTE } from './routes'
import { Join } from './ui/screens/Join'
import { Room } from './ui/screens/Room'

export function App() {
  return (
    <Routes>
      <Route path="/" element={<Join />} />
      <Route path={ROOM_ROUTE} element={<Room />} />
    </Routes>
  )
}
```

В `Screens.module.css` добавить:

```css
.connBanner {
  position: sticky;
  top: 0;
  z-index: 10;
  padding: 0.5rem 1rem;
  text-align: center;
  background: var(--warn-bg, #6b4b16);
  color: var(--warn-fg, #fff);
}

.finishBanner {
  position: fixed;
  inset-inline: 0;
  bottom: 0;
  z-index: 8;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 1rem;
  background: var(--surface, #23232b);
  border-top: 1px solid var(--border, #44444f);
}
```

- [ ] **Step 7: Переписать `web/src/App.test.tsx`**

Существующий тест ходит по удалённому `/room/:code`. Заменить целиком:

```tsx
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { App } from './App'
import { me } from './net/rooms'

vi.mock('./net/rooms', async (importOriginal) => {
  const actual = await importOriginal<typeof import('./net/rooms')>()
  return { ...actual, me: vi.fn(), createRoom: vi.fn(), joinRoom: vi.fn() }
})

afterEach(() => vi.resetAllMocks())

describe('маршруты', () => {
  it('корень — экран входа', () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>,
    )
    expect(screen.getByRole('heading', { name: 'Шух' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Создать комнату/i })).toBeInTheDocument()
  })

  it('/r/CODE — экран комнаты, а не 404 роутера', async () => {
    vi.mocked(me).mockResolvedValue({ kind: 'roomNotFound' })
    render(
      <MemoryRouter initialEntries={['/r/ABCD']}>
        <App />
      </MemoryRouter>,
    )
    expect(await screen.findByText(/Комната не найдена/i)).toBeInTheDocument()
  })
})
```

- [ ] **Step 8: Прогнать гейт и закоммитить**

Run: `cd web && npm run typecheck && npm run lint && npm test`

```bash
git add web/src/App.tsx web/src/App.test.tsx web/src/routes.ts web/src/ui/screens
git commit -m "feat(web): вход по инвайт-ссылке через пробу места + экран комнаты по stage (W3-1/§4)"
```

---

## Task 13: лобби — состав, настройки партии, старт у хоста

**Files:**
- Modify: `web/src/ui/screens/Lobby.tsx`
- Create: `web/src/ui/screens/Lobby.test.tsx`
- Modify: `web/src/transport/ws.ts` (отправка `setConfig`/`start`/`leave`)
- Modify: `web/src/store/game.ts` (проброс команд лобби)

**Interfaces:**
- Produces:
  - `Transport.command(cmd: LobbyCommand): void`, где
    `LobbyCommand = { type: 'setConfig'; config: RoomConfig } | { type: 'start' } | { type: 'leave' }`
    (добавляется в `contract/transport.ts`)
  - `GameState.command: (cmd: LobbyCommand) => void`

- [ ] **Step 1: Написать падающий тест `web/src/ui/screens/Lobby.test.tsx`**

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Lobby } from './Lobby'
import { useGame } from '../../store/GameProvider'
import type { GameState } from '../../store/game'

// Лобби читает стор только через useGame — подменяем его, чтобы не поднимать сокет.
vi.mock('../../store/GameProvider', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../store/GameProvider')>()
  return { ...actual, useGame: vi.fn() }
})

function mockGame(state: Partial<GameState>, command = vi.fn()) {
  const full: GameState = {
    snapshot: {
      roomCode: 'ABCD',
      you: 1,
      stage: 'lobby',
      host: 0,
      seats: [
        { seat: 0, name: 'Вера' },
        { seat: 1, name: 'Боря' },
      ],
      view: null,
      legal: [],
    },
    events: [],
    conn: 'open',
    lastError: null,
    play: vi.fn(),
    command,
    ...state,
  }
  vi.mocked(useGame).mockImplementation((selector) => selector(full))
  return command
}

afterEach(() => vi.resetAllMocks())

describe('лобби', () => {
  it('показывает состав и код комнаты', () => {
    mockGame({})
    render(<Lobby />)
    expect(screen.getByText('ABCD')).toBeInTheDocument()
    expect(screen.getByText('Вера')).toBeInTheDocument()
    expect(screen.getByText('Боря')).toBeInTheDocument()
  })

  it('не-хост не получает кнопку «Начать» (host=0, you=1)', () => {
    mockGame({})
    render(<Lobby />)
    expect(screen.queryByRole('button', { name: /Начать/i })).not.toBeInTheDocument()
  })

  it('хост начинает партию командой start', async () => {
    const command = mockGame({
      snapshot: {
        roomCode: 'ABCD',
        you: 0,
        stage: 'lobby',
        host: 0,
        seats: [
          { seat: 0, name: 'Вера' },
          { seat: 1, name: 'Боря' },
        ],
        view: null,
        legal: [],
      },
    })
    render(<Lobby />)
    await userEvent.click(screen.getByRole('button', { name: /Начать/i }))
    expect(command).toHaveBeenCalledWith({ type: 'start' })
  })

  it('хост меняет колоду — уходит setConfig', async () => {
    const command = mockGame({
      snapshot: {
        roomCode: 'ABCD',
        you: 0,
        stage: 'lobby',
        host: 0,
        seats: [{ seat: 0, name: 'Вера' }],
        view: null,
        legal: [],
      },
    })
    render(<Lobby />)
    await userEvent.selectOptions(screen.getByLabelText('Колода'), '52')
    expect(command).toHaveBeenCalledWith({ type: 'setConfig', config: { deckSize: 52, mode: 'middle' } })
  })
})
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `cd web && npx vitest run src/ui/screens/Lobby.test.tsx`
Expected: FAIL — `command` нет в `GameState`, лобби не рендерит настройки.

- [ ] **Step 3: Добавить команды лобби в контракт и транспорт**

В `web/src/contract/transport.ts`:

```ts
import type { RoomConfig } from '../net/rooms'

// Команды лобби идут тем же сокетом, но это не игровые действия (правил не касаются).
export type LobbyCommand =
  | { type: 'setConfig'; config: RoomConfig }
  | { type: 'start' }
  | { type: 'leave' }
```

и в интерфейс `Transport` добавить:

```ts
  command(cmd: LobbyCommand): void
```

В `web/src/transport/ws.ts` — в возвращаемый объект:

```ts
    command(cmd) {
      if (stopped || status.state !== 'open' || !socket) return
      seq += 1
      const reqId = `c${seq}`
      socket.send(
        JSON.stringify(
          cmd.type === 'setConfig'
            ? { type: 'setConfig', config: cmd.config, reqId }
            : { type: cmd.type, reqId },
        ),
      )
    },
```

В `web/src/store/game.ts` — в интерфейс и фабрику:

```ts
  command: (cmd: LobbyCommand) => void
```
```ts
    command: (cmd) => transport.command(cmd),
```

В тесте стора (`game.test.ts`) добавить `command: () => {}` в инлайновый двойник.

- [ ] **Step 4: Переписать `web/src/ui/screens/Lobby.tsx`**

```tsx
import { useState } from 'react'
import { useGame } from '../../store/GameProvider'
import { selectHost, selectSeats, selectSnapshot, selectYou } from '../../store/game'
import type { RoomConfig } from '../../net/rooms'
import { Button } from '../kit/Button'
import styles from './Screens.module.css'

// Настройки партии живут у хоста (Слой 1 отвергнет их у остальных); роль хоста может
// мигрировать (L2-3), поэтому сравниваем с host из снапшота, а не запоминаем при входе.
//
// Конфиг держим в локальном состоянии: Update Слоя 1 его не несёт, поэтому единственный
// его носитель до старта — выбор самого хоста. Следствие: остальные игроки настроек в
// лобби не видят (зафиксировано как ограничение, не чиним здесь).
export function Lobby() {
  const snapshot = useGame(selectSnapshot)
  const seats = useGame(selectSeats)
  const you = useGame(selectYou)
  const host = useGame(selectHost)
  const command = useGame((s) => s.command)
  const isHost = you !== null && you === host
  const [config, setConfig] = useState<RoomConfig>({ deckSize: 36, mode: 'middle' })

  // Отправляем ВЕСЬ конфиг: SetConfig заменяет его целиком, поэтому частичная посылка
  // молча сбросила бы соседнее поле к дефолту.
  const push = (next: RoomConfig) => {
    setConfig(next)
    command({ type: 'setConfig', config: next })
  }

  return (
    <div className={styles.centered}>
      <h2>
        Комната <span className={styles.code}>{snapshot?.roomCode}</span>
      </h2>
      <p>Позовите друзей — отправьте им адрес этой страницы.</p>
      <ul data-testid="players" className={styles.players}>
        {seats.map((s) => (
          <li key={s.seat}>
            {s.name}
            {s.seat === host ? ' — хост' : ''}
          </li>
        ))}
      </ul>
      {isHost && (
        <>
          <label>
            Колода
            <select
              value={String(config.deckSize)}
              onChange={(e) => push({ ...config, deckSize: Number(e.target.value) as 36 | 52 })}
            >
              <option value="36">36 карт</option>
              <option value="52">52 карты</option>
            </select>
          </label>
          <label>
            Строгость
            <select
              value={config.mode}
              onChange={(e) => push({ ...config, mode: e.target.value as RoomConfig['mode'] })}
            >
              <option value="guard">Максимум защиты</option>
              <option value="middle">Середина</option>
            </select>
          </label>
          <Button onClick={() => command({ type: 'start' })} disabled={seats.length < 2}>
            Начать
          </Button>
        </>
      )}
      {!isHost && <p>Ждём, пока хост начнёт партию…</p>}
    </div>
  )
}
```

Примечание: режим `culture` в списке отсутствует намеренно — движок его не реализует (D-10).

- [ ] **Step 5: Прогнать гейт и закоммитить**

Run: `cd web && npm run typecheck && npm run lint && npm test`

```bash
git add web/src/ui/screens/Lobby.tsx web/src/ui/screens/Lobby.test.tsx web/src/contract/transport.ts web/src/transport/ws.ts web/src/store/game.ts web/src/store/game.test.ts
git commit -m "feat(web): лобби — состав, настройки партии и старт у хоста"
```

---

## Task 14: ActionBar — недостающие действия

**Files:**
- Modify: `web/src/ui/table/ActionBar.tsx`, `web/src/ui/screens/Table.tsx`
- Test: `web/src/ui/table/ActionBar.test.tsx`

**Interfaces:**
- Produces: `ActionBar` с пропсами
  `{ actions: { label: string; enabled: boolean; onClick: () => void; pulse?: boolean }[] }` —
  панель перестаёт знать про конкретные действия, список собирает `Table.tsx` из `legal`.

- [ ] **Step 1: Переписать тест `web/src/ui/table/ActionBar.test.tsx`**

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ActionBar } from './ActionBar'

describe('ActionBar', () => {
  it('рендерит переданные действия и гасит недоступные', async () => {
    const onPlay = vi.fn()
    render(
      <ActionBar
        actions={[
          { label: 'Сходить', enabled: true, onClick: onPlay },
          { label: 'Сбросить Запад', enabled: false, onClick: vi.fn() },
        ]}
      />,
    )
    await userEvent.click(screen.getByRole('button', { name: 'Сходить' }))
    expect(onPlay).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('button', { name: 'Сбросить Запад' })).toBeDisabled()
  })
})
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `cd web && npx vitest run src/ui/table/ActionBar.test.tsx`
Expected: FAIL — компонент принимает старые пропсы.

- [ ] **Step 3: Переписать `web/src/ui/table/ActionBar.tsx`**

```tsx
import { Button } from '../kit/Button'
import { cx } from '../kit/cx'
import styles from './Table.module.css'

export interface BarAction {
  label: string
  enabled: boolean
  onClick: () => void
  pulse?: boolean
}

// Панель ничего не знает о правилах: список действий собирает стол из snapshot.legal (W2-2).
export function ActionBar({ actions }: { actions: BarAction[] }) {
  return (
    <div className={styles.actionBar} data-testid="action-bar">
      {actions.map((a) => (
        <Button
          key={a.label}
          onClick={a.onClick}
          disabled={!a.enabled}
          className={cx(a.pulse && a.enabled && styles.pulse)}
        >
          {a.label}
        </Button>
      ))}
    </div>
  )
}
```

- [ ] **Step 4: Собрать полный список действий в `web/src/ui/screens/Table.tsx`**

Добавить импорт `import { ActionBar, type BarAction } from '../table/ActionBar'`, затем заменить
блок `<ActionBar .../>` и подготовку флагов:

```tsx
  const declareOneCard = legal.find((a) => a.type === 'declareOneCard')
  const barActions: BarAction[] = [
    { label: 'Сходить', enabled: canConfirm, onClick: confirmPlay },
    {
      label: 'Взять низ',
      enabled: canTakeBottom,
      onClick: () => play({ type: 'takeBottomAndPass' }),
    },
    {
      label: 'Западло',
      enabled: isLegal(legal, { type: 'podkladkaWest' }),
      onClick: () => play({ type: 'podkladkaWest' }),
    },
    {
      // R-9.4.2.1: в эндшпиле §9.2 сброс 6(2)♥ — обязательный ход, без него стол встаёт.
      label: 'Сбросить Запад',
      enabled: isLegal(legal, { type: 'discardWest' }),
      onClick: () => play({ type: 'discardWest' }),
    },
    { label: 'ШУХ!', enabled: claim != null, onClick: () => claim && play(claim) },
    {
      // Настоящее объявление §6, а не клиент-локальный флаг: теперь Ш-11 ловится по правилам.
      label: 'Одна карта!',
      enabled: declareOneCard != null,
      onClick: () => declareOneCard && play(declareOneCard),
      pulse: true,
    },
  ]
```

и рендер: `<ActionBar actions={barActions} />`.

Удалить состояние `announced` и связанный с ним `useEffect` — объявление теперь живёт в `legal`.

- [ ] **Step 5: Прогнать гейт и закоммитить**

Run: `cd web && npm run typecheck && npm run lint && npm test`

```bash
git add web/src/ui/table/ActionBar.tsx web/src/ui/table/ActionBar.test.tsx web/src/ui/screens/Table.tsx
git commit -m "feat(web): полный набор своих действий — Западло, Сбросить Запад, настоящая «Одна карта!»"
```

---

## Task 15: меню адресных действий на месте соперника

**Files:**
- Create: `web/src/ui/table/SeatMenu.tsx`, `web/src/ui/table/SeatMenu.test.tsx`
- Modify: `web/src/ui/table/OpponentSeat.tsx`, `web/src/ui/screens/Table.tsx`,
  `web/src/ui/table/Table.module.css`

**Interfaces:**
- Produces: `SeatMenu({ seat, name, legal, you, onAction, onClose })` — список пунктов; адресные
  из `legal`, субъективные ШУХи всегда.

- [ ] **Step 1: Написать падающий тест `web/src/ui/table/SeatMenu.test.tsx`**

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SeatMenu } from './SeatMenu'
import type { Action } from '../../contract/types'

describe('меню соперника', () => {
  it('скрывает адресные вопросы, которых нет в legal', () => {
    render(<SeatMenu seat={1} name="Боря" you={0} legal={[]} onAction={vi.fn()} onClose={vi.fn()} />)
    expect(screen.queryByRole('button', { name: /Сколько карт/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Есть Запад/i })).not.toBeInTheDocument()
  })

  it('показывает вопрос о картах, когда он легален (R-6)', async () => {
    const onAction = vi.fn()
    const legal: Action[] = [{ type: 'askCount', target: 1 }]
    render(<SeatMenu seat={1} name="Боря" you={0} legal={legal} onAction={onAction} onClose={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: /Сколько карт/i }))
    expect(onAction).toHaveBeenCalledWith({ type: 'askCount', target: 1 })
  })

  it('субъективные ШУХи доступны всегда — движок их в legal не кладёт', async () => {
    const onAction = vi.fn()
    render(<SeatMenu seat={1} name="Боря" you={0} legal={[]} onAction={onAction} onClose={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: /завис/i }))
    expect(onAction).toHaveBeenCalledWith({ type: 'claimSubjective', claimant: 0, target: 1, code: 6 })
  })

  it('Esc закрывает меню', async () => {
    const onClose = vi.fn()
    render(<SeatMenu seat={1} name="Боря" you={0} legal={[]} onAction={vi.fn()} onClose={onClose} />)
    await userEvent.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalled()
  })
})
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `cd web && npx vitest run src/ui/table/SeatMenu.test.tsx`
Expected: FAIL — модуль `./SeatMenu` не найден.

- [ ] **Step 3: Написать `web/src/ui/table/SeatMenu.tsx`**

```tsx
import { useEffect } from 'react'
import { isLegal } from '../../contract/types'
import type { Action, SeatID, ShukhCode } from '../../contract/types'
import { Button } from '../kit/Button'
import styles from './Table.module.css'

interface SeatMenuProps {
  seat: SeatID
  name: string
  you: SeatID
  legal: Action[]
  onAction: (a: Action) => void
  onClose: () => void
}

// Субъективные ШУХи (R-8.4/R-8.7/R-8.8) движок в legal НЕ перечисляет — это всегда
// доступная социальная кнопка, законность которой сервер проверяет на сабмите.
const SUBJECTIVE: { code: ShukhCode; label: string }[] = [
  { code: 6, label: 'ШУХ: завис (Ш-6)' },
  { code: 9, label: 'ШУХ: зря крикнул (Ш-9)' },
  { code: 10, label: 'ШУХ: небрежность (Ш-10)' },
]

export function SeatMenu({ seat, name, you, legal, onAction, onClose }: SeatMenuProps) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const fire = (a: Action) => {
    onAction(a)
    onClose()
  }
  const askCount: Action = { type: 'askCount', target: seat }
  const askWest: Action = { type: 'askAboutWest', target: seat }

  return (
    <div className={styles.seatMenu} role="menu" aria-label={`Действия: ${name}`}>
      {isLegal(legal, askCount) && (
        <Button role="menuitem" onClick={() => fire(askCount)}>
          Сколько карт?
        </Button>
      )}
      {isLegal(legal, askWest) && (
        <Button role="menuitem" onClick={() => fire(askWest)}>
          Есть Запад?
        </Button>
      )}
      {SUBJECTIVE.map((s) => (
        <Button
          key={s.code}
          role="menuitem"
          onClick={() => fire({ type: 'claimSubjective', claimant: you, target: seat, code: s.code })}
        >
          {s.label}
        </Button>
      ))}
    </div>
  )
}
```

- [ ] **Step 4: Подключить меню к месту соперника**

`OpponentSeat.tsx` — сделать место кликабельным и показывать меню:

```tsx
interface OpponentSeatProps {
  name: string
  opponent: OpponentView
  menuOpen: boolean
  onToggleMenu: () => void
  children?: ReactNode // меню, когда открыто
}
```

Обернуть заголовок места в кнопку:

```tsx
      <button type="button" className={styles.seatName} onClick={onToggleMenu} aria-expanded={menuOpen}>
        {name}
      </button>
```

и рендерить `{menuOpen && children}`.

В `Table.tsx` завести состояние `const [menuSeat, setMenuSeat] = useState<number | null>(null)` и
передать в каждый `OpponentSeat`:

```tsx
        {view.opponents.map((o) => (
          <OpponentSeat
            key={o.seat}
            name={nameOf(o.seat)}
            opponent={o}
            menuOpen={menuSeat === o.seat}
            onToggleMenu={() => setMenuSeat(menuSeat === o.seat ? null : o.seat)}
          >
            <SeatMenu
              seat={o.seat}
              name={nameOf(o.seat)}
              you={view.you}
              legal={legal}
              onAction={play}
              onClose={() => setMenuSeat(null)}
            />
          </OpponentSeat>
        ))}
```

В `Table.module.css` добавить (место `.seat` уже позиционировано относительно — если нет, добавить
ему `position: relative`):

```css
.seatMenu {
  position: absolute;
  top: 100%;
  left: 0;
  z-index: 5;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.5rem;
  min-width: 12rem;
  background: var(--surface, #23232b);
  border: 1px solid var(--border, #44444f);
  border-radius: 0.5rem;
  box-shadow: 0 0.5rem 1rem rgb(0 0 0 / 0.35);
}
```

- [ ] **Step 5: Прогнать гейт и закоммитить**

Run: `cd web && npm run typecheck && npm run lint && npm test`

```bash
git add web/src/ui/table/SeatMenu.tsx web/src/ui/table/SeatMenu.test.tsx web/src/ui/table/OpponentSeat.tsx web/src/ui/table/OpponentSeat.test.tsx web/src/ui/screens/Table.tsx web/src/ui/table/Table.module.css
git commit -m "feat(web): меню соперника — askCount/askAboutWest/claimSubjective (§6)"
```

---

## Task 16: модалка голосования на реальных данных

**Files:**
- Modify: `web/src/ui/table/ShukhVoteModal.tsx`, `web/src/ui/table/ShukhVoteModal.test.tsx`
- Modify: `web/src/ui/screens/Table.tsx`

**Interfaces:**
- Consumes: `selectVote`/`selectVoteDeadline`/`selectEvents` (Task 11), `VoteView` (Task 7).
- Produces: `ShukhVoteModal({ vote, deadline, legal, nameOf, onVote, outcome })`, где
  `outcome?: { code: ShukhCode; overturned: boolean }`.

- [ ] **Step 1: Переписать тест `web/src/ui/table/ShukhVoteModal.test.tsx`**

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ShukhVoteModal } from './ShukhVoteModal'
import type { Action, VoteView } from '../../contract/types'

const vote: VoteView = { claimant: 0, target: 1, code: 6, voted: [0] }
const nameOf = (s: number) => ['Вера', 'Боря', 'Гена'][s] ?? `Игрок ${s}`

describe('модалка голосования R-8.6', () => {
  it('показывает предмет разбора и КТО проголосовал, но не КАК (§8.4)', () => {
    render(<ShukhVoteModal vote={vote} deadline={null} legal={[]} nameOf={nameOf} onVote={vi.fn()} />)
    expect(screen.getByText(/Боря/)).toBeInTheDocument()
    expect(screen.getByText(/Ш-6/)).toBeInTheDocument()
    expect(screen.getByTestId('voted-0')).toHaveTextContent('Вера')
    expect(screen.queryByText(/за|против/i)).not.toBeInTheDocument()
  })

  it('кнопки голоса появляются только когда голос легален', async () => {
    const onVote = vi.fn()
    const { rerender } = render(
      <ShukhVoteModal vote={vote} deadline={null} legal={[]} nameOf={nameOf} onVote={onVote} />,
    )
    expect(screen.queryByRole('button', { name: /За ШУХ/i })).not.toBeInTheDocument()

    const legal: Action[] = [
      { type: 'vote', vote: 'forShukh' },
      { type: 'vote', vote: 'againstShukh' },
    ]
    rerender(<ShukhVoteModal vote={vote} deadline={null} legal={legal} nameOf={nameOf} onVote={onVote} />)
    await userEvent.click(screen.getByRole('button', { name: /Против ШУХа/i }))
    expect(onVote).toHaveBeenCalledWith('againstShukh')
  })

  it('исход берётся из события voteResolved', () => {
    render(
      <ShukhVoteModal
        vote={vote}
        deadline={null}
        legal={[]}
        nameOf={nameOf}
        onVote={vi.fn()}
        outcome={{ code: 8, overturned: true }}
      />,
    )
    expect(screen.getByTestId('vote-outcome')).toHaveTextContent(/отклонён/i)
  })
})
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `cd web && npx vitest run src/ui/table/ShukhVoteModal.test.tsx`
Expected: FAIL — компонент ждёт удалённый тип `ShukhVote`.

- [ ] **Step 3: Переписать `web/src/ui/table/ShukhVoteModal.tsx`**

```tsx
import { useEffect, useState } from 'react'
import { isLegal } from '../../contract/types'
import type { Action, ShukhCode, VoteView } from '../../contract/types'
import { Button } from '../kit/Button'
import styles from './Table.module.css'

interface ShukhVoteModalProps {
  vote: VoteView
  deadline: number | null
  legal: Action[]
  nameOf: (seat: number) => string
  onVote: (v: 'forShukh' | 'againstShukh') => void
  outcome?: { code: ShukhCode; overturned: boolean }
}

// Разбор R-8.6. Открывается по view.vote (W3-6), поэтому переподключившийся сразу видит
// идущее голосование. Показываем ФАКТ голоса, но не содержание — бюллетень тайный (§8.4).
export function ShukhVoteModal({ vote, deadline, legal, nameOf, onVote, outcome }: ShukhVoteModalProps) {
  const [left, setLeft] = useState(() => remaining(deadline))
  useEffect(() => {
    if (deadline === null) return
    setLeft(remaining(deadline))
    const id = setInterval(() => setLeft(remaining(deadline)), 1000)
    return () => clearInterval(id)
  }, [deadline])

  const forShukh: Action = { type: 'vote', vote: 'forShukh' }
  const against: Action = { type: 'vote', vote: 'againstShukh' }
  const canVote = isLegal(legal, forShukh) || isLegal(legal, against)

  return (
    <div
      className={styles.modalBackdrop}
      role="dialog"
      aria-modal="true"
      aria-label="Голосование по ШУХу"
      data-testid="shukh-vote"
    >
      <div className={styles.modal}>
        <h3>
          ШУХ на «{nameOf(vote.target)}» (Ш-{vote.code})
        </h3>
        <p>Предъявил: {nameOf(vote.claimant)}</p>
        <ul className={styles.voteList}>
          {vote.voted.map((seat) => (
            <li key={seat} data-testid={`voted-${seat}`}>
              {nameOf(seat)}: голос отдан
            </li>
          ))}
        </ul>
        {outcome ? (
          <p className={styles.voteOutcome} data-testid="vote-outcome">
            {outcome.overturned ? 'ШУХ отклонён — Ш-8 предъявившему' : 'ШУХ подтверждён'}
          </p>
        ) : canVote ? (
          <div className={styles.voteButtons}>
            <Button onClick={() => onVote('forShukh')}>За ШУХ</Button>
            <Button onClick={() => onVote('againstShukh')}>Против ШУХа</Button>
          </div>
        ) : (
          <p className={styles.voteTallying}>
            {left === null ? 'Голосование…' : left > 0 ? `Ждём остальных: ${left} с` : 'Подводим итог…'}
          </p>
        )}
      </div>
    </div>
  )
}

function remaining(deadline: number | null): number | null {
  if (deadline === null) return null
  return Math.max(0, Math.ceil((deadline - Date.now()) / 1000))
}
```

- [ ] **Step 4: Подключить в `web/src/ui/screens/Table.tsx`**

```tsx
  const vote = useGame(selectVote)
  const voteDeadline = useGame(selectVoteDeadline)
  const events = useGame(selectEvents)
  // Исход показывает последнее voteResolved — состояние разбора уже закрыто, событие живёт в логе.
  const lastResolved = [...events].reverse().find((e) => e.type === 'voteResolved')
```
```tsx
      {vote && (
        <ShukhVoteModal
          vote={vote}
          deadline={voteDeadline}
          legal={legal}
          nameOf={nameOf}
          onVote={(v) => play({ type: 'vote', vote: v })}
          outcome={
            lastResolved && lastResolved.type === 'voteResolved'
              ? { code: lastResolved.code, overturned: lastResolved.overturned }
              : undefined
          }
        />
      )}
```

Также заменить в `Table.tsx` все `useGameStore(...)` на `useGame(...)` из `store/GameProvider`.

В `Table.module.css` добавить:

```css
.voteButtons {
  display: flex;
  gap: 0.75rem;
  justify-content: center;
  margin-top: 0.75rem;
}
```

- [ ] **Step 5: Прогнать гейт и закоммитить**

Run: `cd web && npm run typecheck && npm run lint && npm test`

```bash
git add web/src/ui/table/ShukhVoteModal.tsx web/src/ui/table/ShukhVoteModal.test.tsx web/src/ui/screens/Table.tsx web/src/ui/table/Table.module.css
git commit -m "feat(web): голосование R-8.6 на реальных данных — VoteView, отсчёт, отправка голоса"
```

---

## Task 17: уборка двойников и приёмка

**Files:**
- Delete: `web/src/transport/scripted.ts`, `web/src/transport/scripted.test.ts`,
  `web/src/transport/mock.ts`, `web/src/transport/mock.test.ts`,
  `web/src/fixtures/scenario.ts`, `web/src/fixtures/scenario.test.ts`
- Modify: `docs/architecture.md` (журнал изменений, ревизия D-2, OQ-2/OQ-4)
- Modify: `README.md` (как запустить оба процесса)

- [ ] **Step 1: Удалить двойники и убедиться, что на них никто не ссылается**

```bash
git rm web/src/transport/scripted.ts web/src/transport/scripted.test.ts \
       web/src/transport/mock.ts web/src/transport/mock.test.ts \
       web/src/fixtures/scenario.ts web/src/fixtures/scenario.test.ts
grep -rn 'scripted\|demoScenario\|createMockTransport' web/src || echo "ссылок нет"
```

Expected: «ссылок нет». Если что-то найдено — починить импорт (фикстуры `fixtures/game.ts` и
`fixtures/seatView.ts` остаются, они кормят тесты компонентов).

- [ ] **Step 2: Прогнать полный гейт с обеих сторон**

Run: `cd web && npm run typecheck && npm run lint && npm test`
Run: `cd .. && go build ./... && go test ./...`
Expected: всё зелёное.

- [ ] **Step 3: Ручная приёмка партии на двоих**

Терминал 1: `go run ./cmd/shukh-server -origins http://localhost:5173`
Терминал 2: `cd web && npm run dev`

Пройти по списку и убедиться, что каждый пункт работает:
1. Открыть `http://localhost:5173`, ввести имя, «Создать комнату» → попадаем в лобби с кодом.
2. Скопировать адрес `/r/CODE`, открыть во втором браузерном профиле (или приватном окне) →
   форма имени → «Занять место» → оба видят состав из двух игроков.
3. Хост меняет колоду на 52 и обратно на 36 → изменение не роняет лобби.
4. «Начать» → у **обоих** экран переключается на стол без перезагрузки (W3-1).
5. Сходить картой, взять низ — подсветка легальных ходов совпадает с тем, что принимает сервер.
6. Довести одного игрока до одной карты, нажать «Одна карта!»; вторым игроком поймать
   отсутствие объявления кнопкой «ШУХ!».
7. Через меню соперника предъявить субъективный ШУХ (Ш-6) → у обоих открывается модалка,
   виден отсчёт, голоса отображаются как факт; проголосовать с обеих сторон → исход показан.
8. Закрыть вкладку посреди партии и открыть `/r/CODE` заново → стол восстановился, ход не потерян.
9. Доиграть до конца — включая эндшпиль на двоих со «Сбросить Запад», если он выпал.

- [ ] **Step 4: Обновить `README.md`**

Добавить раздел «Запуск локально» с двумя командами из Step 3 и пояснением, что `-origins`
обязателен, потому что фронт и сервер живут на разных origin (W3-4).

- [ ] **Step 5: Обновить `docs/architecture.md`**

В §7 «Журнал изменений» добавить:

```markdown
- **2026-08-02.** Завершена итерация 3 Спеца 3: веб-клиент сшит с сервером. Контракт
  синхронизирован (§7.4 Слоя 2 закрыт): 12 действий, 17 событий, `VoteView`, `stage`/`host`/`you`.
  Новое на клиенте — `contract/wire.ts`, `net/rooms.ts`, `transport/ws.ts`, стор на комнату;
  экран комнаты ветвится по `stage`, а не по URL (W3-1). API переехал на `/api` + `/ws`, адрес
  `/r/CODE` освободился под инвайт-ссылку (W3-2, D-2 дословно). Расхождение ручных зеркал ловят
  golden-фикстуры протокола (W3-3). Раздельные origin + CORS (W3-4) — попутно закрыт
  `TODO(prod)` про `InsecureSkipVerify` (WS `OriginPatterns`). Скриптованный транспорт и
  демо-сценарий удалены (W3-7).
```

В §5 «Открытые вопросы» отметить, что OQ-2/OQ-4 стали ближайшим кандидатом на следующий спек:
сетевая партия рвётся на первом же навсегда ушедшем игроке.

- [ ] **Step 6: Коммит**

```bash
git add -A
git commit -m "chore(web,docs): удалены двойники транспорта, README и журнал архитектуры обновлены"
```

---

## Порядок и зависимости

Задачи 1–6 (Go) не зависят от web и могут идти параллельно с 7, но **Task 8 требует фикстур из
Task 6**. Внутри web порядок строгий: 7 → 8 → 9 → 10 → 11 → 12 → 13 → 14 → 15 → 16 → 17.

**Гейт зелёный в конце КАЖДОЙ задачи, без исключений** (Global Constraints). Task 7 сносит
`ShukhVote` и `SeatMeta.ready`, на которые опираются модалка голосования и лобби, поэтому она же
обязана оставить их компилируемыми — временной заглушкой и удалением устаревших тестов модалки
(Task 16 пишет их заново на реальных данных). Каждый коммит ветки остаётся рабочим.
