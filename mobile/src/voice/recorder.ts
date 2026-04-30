import Voice from "@react-native-voice/voice";

export class Recorder {
  private listeners: Array<(text: string) => void> = [];
  private errs: Array<(e: Error) => void> = [];
  private locale: string;

  constructor(locale = "en-US") {
    this.locale = locale;
    Voice.onSpeechResults = (e: any) => {
      const t = e?.value?.[0];
      if (typeof t === "string") this.listeners.forEach((cb) => cb(t));
    };
    Voice.onSpeechError = (e: any) => {
      const msg = e?.error?.message ?? "speech error";
      this.errs.forEach((cb) => cb(new Error(msg)));
    };
  }

  onResult(cb: (text: string) => void) {
    this.listeners.push(cb);
  }
  onError(cb: (e: Error) => void) {
    this.errs.push(cb);
  }

  async start() {
    await Voice.start(this.locale);
  }
  async stop() {
    await Voice.stop();
  }
  async destroy() {
    await Voice.destroy();
    Voice.removeAllListeners();
  }
}
