import Voice from "@react-native-voice/voice";
import { Recorder } from "../recorder";

jest.mock("@react-native-voice/voice", () => ({
  __esModule: true,
  default: {
    onSpeechResults: null as any,
    onSpeechError: null as any,
    start: jest.fn(),
    stop: jest.fn(),
    destroy: jest.fn(),
    removeAllListeners: jest.fn(),
  },
}));

beforeEach(() => {
  (Voice as any).start.mockClear();
  (Voice as any).stop.mockClear();
});

test("start triggers Voice.start, transcripts reach onResult", async () => {
  const r = new Recorder();
  const got: string[] = [];
  r.onResult((t) => got.push(t));
  await r.start();
  expect((Voice as any).start).toHaveBeenCalled();
  (Voice as any).onSpeechResults?.({ value: ["hello world"] });
  expect(got).toEqual(["hello world"]);
});

test("stop calls Voice.stop", async () => {
  const r = new Recorder();
  await r.stop();
  expect((Voice as any).stop).toHaveBeenCalled();
});

test("speech errors reach onError", () => {
  const r = new Recorder();
  const errs: Error[] = [];
  r.onError((e) => errs.push(e));
  (Voice as any).onSpeechError?.({ error: { message: "mic blocked" } });
  expect(errs).toHaveLength(1);
  expect(errs[0]!.message).toContain("mic blocked");
});
