import { WSTransport } from "./transport";
import { ACPFrame, encodeFrame, isResponse, isServerRequest } from "./types";

type Pending = { resolve: (v: any) => void; reject: (e: Error) => void };
type ServerHandler = (
  method: string,
  params: any,
  reply: (result: any) => void,
  fail: (code: number, message: string) => void,
) => void | Promise<void>;

export class ACPClient {
  private nextId = 1;
  private pending = new Map<number | string, Pending>();
  private handlers = new Map<string, ServerHandler>();
  private notifyListeners: ((f: ACPFrame) => void)[] = [];

  constructor(private transport: WSTransport) {
    transport.onFrame((f) => this.dispatch(f));
  }

  request<T = unknown>(method: string, params?: unknown): Promise<T> {
    const id = this.nextId++;
    const frame = encodeFrame({ id, method, params: params as any });
    return new Promise<T>((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.transport.send(frame);
    });
  }

  notify(method: string, params?: unknown): void {
    this.transport.send(encodeFrame({ method, params: params as any }));
  }

  onServerRequest(method: string, h: ServerHandler): () => void {
    this.handlers.set(method, h);
    return () => {
      this.handlers.delete(method);
    };
  }

  onNotification(cb: (f: ACPFrame) => void): () => void {
    this.notifyListeners.push(cb);
    return () => {
      this.notifyListeners = this.notifyListeners.filter((x) => x !== cb);
    };
  }

  private dispatch(f: ACPFrame) {
    if (isResponse(f) && f.id !== undefined) {
      const p = this.pending.get(f.id);
      if (!p) return;
      this.pending.delete(f.id);
      if (f.error) p.reject(new Error(f.error.message));
      else p.resolve(f.result);
      return;
    }
    if (isServerRequest(f) && f.method && f.id !== undefined) {
      const id = f.id;
      const reply = (result: any) => this.transport.send(encodeFrame({ id, result }));
      const fail = (code: number, message: string) =>
        this.transport.send(encodeFrame({ id, error: { code, message } }));
      const h = this.handlers.get(f.method);
      if (!h) {
        fail(-32601, `no handler for ${f.method}`);
        return;
      }
      Promise.resolve(h(f.method, f.params, reply, fail)).catch((e) =>
        fail(-32000, String(e)),
      );
      return;
    }
    this.notifyListeners.forEach((cb) => cb(f));
  }
}
