import socket, ssl, base64, os, struct, time, sys

HOST = "gateway.24alert.ru"
PORT = 8080
PATH = "/api/v1/stream/orderbook?uids=test-uid&depth=20"

def recv_frame(s):
    hdr = b''
    while len(hdr) < 2:
        hdr += s.recv(2 - len(hdr))
    b1, b2 = hdr[0], hdr[1]
    opcode = b1 & 0x0f
    masked = b2 & 0x80
    length = b2 & 0x7f
    if length == 126:
        ext = b''
        while len(ext) < 2: ext += s.recv(2 - len(ext))
        length = struct.unpack('>H', ext)[0]
    elif length == 127:
        ext = b''
        while len(ext) < 8: ext += s.recv(8 - len(ext))
        length = struct.unpack('>Q', ext)[0]
    if masked:
        s.recv(4)
    payload = b''
    while len(payload) < length:
        payload += s.recv(length - len(payload))
    return opcode, payload

ctx = ssl.create_default_context()
raw = socket.create_connection((HOST, PORT), timeout=10)
s = ctx.wrap_socket(raw, server_hostname=HOST)
key = base64.b64encode(os.urandom(16)).decode()
req = (
    f"GET {PATH} HTTP/1.1\r\n"
    f"Host: {HOST}:{PORT}\r\n"
    f"Upgrade: websocket\r\n"
    f"Connection: Upgrade\r\n"
    f"Sec-WebSocket-Key: {key}\r\n"
    f"Sec-WebSocket-Version: 13\r\n\r\n"
)
s.send(req.encode())
s.settimeout(20.0)
resp = b''
while b'\r\n\r\n' not in resp:
    chunk = s.recv(2048)
    if not chunk:
        break
    resp += chunk
print("HANDSHAKE:", resp.split(b"\r\n", 1)[0].decode())
count = 0
start = time.time()
while count < 2 and time.time() - start < 25:
    try:
        op, pl = recv_frame(s)
    except socket.timeout:
        print("...timeout, waiting")
        continue
    count += 1
    if op == 0x1:
        print(f"TEXT[{len(pl)}B]:", pl[:300].decode(errors="replace"))
    elif op == 0x8:
        print("CLOSE"); break
    else:
        print(f"OP=0x{op:x} len={len(pl)}")
print("DONE")
s.close()
