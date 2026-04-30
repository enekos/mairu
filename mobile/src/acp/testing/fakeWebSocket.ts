type Listener = (ev: any) => void;

export class FakeWebSocket {
  static instances: FakeWebSocket[] = [];
  url: string;
  readyState = 0;
  sent: string[] = [];
  listeners: Record<string, Listener[]> = { open: [], message: [], close: [], error: [] };

  constructor(url: string, _protocols?: string | string[]) {
    this.url = url;
    FakeWebSocket.instances.push(this);
  }

  addEventListener(t: string, l: Listener) {
    (this.listeners[t] ?? (this.listeners[t] = [])).push(l);
  }
  removeEventListener(t: string, l: Listener) {
    this.listeners[t] = (this.listeners[t] ?? []).filter((x) => x !== l);
  }
  send(data: string) {
    this.sent.push(data);
  }
  close() {
    this.readyState = 3;
    this.fire("close", { code: 1000 });
  }

  fire(t: string, ev: any) {
    (this.listeners[t] ?? []).forEach((l) => l(ev));
  }

  // test helpers
  open() {
    this.readyState = 1;
    this.fire("open", {});
  }
  recv(data: string) {
    this.fire("message", { data });
  }
  fail(code = 1006) {
    this.readyState = 3;
    this.fire("close", { code });
  }
}
