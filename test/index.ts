import {
  connect, JSONCodec,
  type NatsConnection,
  type Codec
} from "nats";
import { v4 as uuidv4 } from "uuid"; // For generating unique IDs
import { Database } from "bun:sqlite";
import fs from "fs";

interface TestCaseResult {
  //recieived from worker SUBJECT: submission.result
  submissionId: string;
  testCaseId: string;
  status: string;
  timeUsedInMs: number;
  memoryUsedInKb: number;
  output: string;
  error: string;
}
const db = new Database("test.db");
db.query(
  "CREATE TABLE IF NOT EXISTS testcases (subId TEXT, testId , input TEXT, expectOutput TEXT,fileName TEXT,time TEXT,memory TEXT,output TEXT,error TEXT,status TEXT)"
).run();
// --- Configuration ---
const NATS_URL = process.env.NATS_URL || "nats://localhost:4222";
const SUBMISSION_CREATED_SUBJECT = "submission.created";
const PUBLISH_INTERVAL_MS = 1000; // Publish a new submission every 3 seconds

// --- TypeScript Interfaces (mirroring Go models) ---
interface TestCase {
  id: string;
  input: string;
  expectOutput: string;
}

interface Language {
  id: string;
  sourceFile: string;
  binaryFile: string; // Corresponds to BinaryFile in Go struct
  compileCommand: string;
  runCommand: string;
}

interface SubmissionSettings {
  withTrim: boolean;
  withCaseSensitive: boolean;
  withWhitespace: boolean;
}

interface Submission {
  id: string;
  language: Language;
  code: string;
  timeLimitInMs: number;
  memoryLimitInKb: number;
  testCases: TestCase[];
  settings: SubmissionSettings;
}
const testcases = JSON.parse(
  await Bun.file("./testCases.json").text()
) as unknown as TestCase[];
const codeDir = "./code_test";
const goLanguage: Language = {
  id: "go",
  sourceFile: "main.go",
  binaryFile: "main.exe", // Output of 'go build -o main main.go'
  compileCommand: "go build -o main.exe main.go",
  runCommand: "./main.exe",
};
const allCodeSnippets = fs
  .readdirSync(codeDir)
  .filter((file) => file.endsWith(".go"))
  .map<Submission>((file) => {
    const content = fs.readFileSync(`${codeDir}/${file}`, "utf-8");

    return {
      id: uuidv4(),
      fileName: file,
      language: goLanguage,
      code: content,
      timeLimitInMs: 2000,
      memoryLimitInKb: 256 * 1024, // 256 MB
      testCases: testcases,
      settings: {
        withTrim: true,
        withCaseSensitive: true,
        withWhitespace: true,
      },
    };
  });

// --- Helper Functions ---

function getRandomElement<T>(arr: T[]): T {
  return arr[Math.floor(Math.random() * arr.length)]!;
}

function createSampleSubmission(): Submission {
  const selected = getRandomElement<Submission>(allCodeSnippets);
  return selected;
}

// --- Main Application Logic ---

async function runPublisher() {
  let nc: NatsConnection;
  try {
    nc = await connect({ servers: NATS_URL });
    console.log(`Connected to NATS server at ${nc.getServer()}`);
  } catch (err) {
    console.error("Error connecting to NATS:", err);
    process.exit(1);
  }

  const jsonCodec: Codec<Submission> = JSONCodec<Submission>(); // Specify the type for the codec

  console.log(
    `Starting to publish submissions to '${SUBMISSION_CREATED_SUBJECT}' every ${PUBLISH_INTERVAL_MS}ms...`
  );
  const insert = db.prepare(
    "INSERT INTO testcases (subId, testId, input, expectOutput, fileName, time, memory, output, error, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
  );
  const intervalId = setInterval(() => {
    const submission = createSampleSubmission();
    try {
      nc.publish(SUBMISSION_CREATED_SUBJECT, jsonCodec.encode(submission));
      for (const testCase of submission.testCases) {
        insert.run(
          submission.id,
          testCase.id,
          testCase.input,
          testCase.expectOutput,
          submission.fileName,
          "",
          "",
          "",
          "",
          "pending"
        );
        console.log(
          `Published submission ${submission.id} with test case ${testCase.id}`
        );
      }
    } catch (err) {
      console.error(`Error publishing submission ${submission.id}:`, err);
    }
  }, PUBLISH_INTERVAL_MS);

  // Graceful shutdown
  process.on("SIGINT", async () => {
    console.log("\nCaught interrupt signal. Draining NATS connection...");
    clearInterval(intervalId);
    await nc.drain();
    console.log("NATS connection drained. Exiting.");
    process.exit(0);
  });

  process.on("SIGTERM", async () => {
    console.log("\nCaught terminate signal. Draining NATS connection...");
    clearInterval(intervalId);
    await nc.drain();
    console.log("NATS connection drained. Exiting.");
    process.exit(0);
  });
}

runPublisher().catch((err) => {
  console.error("Unhandled error in publisher:", err);
  process.exit(1);
});
const RESULT_SUBJECT = "submission.executed";

const update = db.prepare(`
  UPDATE testcases
  SET time = ?, memory = ?, output = ?, error = ?, status = ?
  WHERE subId = ? AND testId = ?
`);

async function runSubscriber() {
  const nc = await connect({ servers: NATS_URL });
  console.log(`Connected to NATS at ${nc.getServer()}`);

  const jsonCodec: Codec<TestCaseResult> = JSONCodec<TestCaseResult>();

  const sub = nc.subscribe(RESULT_SUBJECT);
  console.log(`Listening for results on '${RESULT_SUBJECT}'...`);

  for await (const msg of sub) {
    try {
      const result = jsonCodec.decode(msg.data);
      update.run(
        `${result.timeUsedInMs}`,
        `${result.memoryUsedInKb}`,
        result.output,
        result.error,
        result.status,
        result.submissionId,
        result.testCaseId
      );
      console.log(
        `✔️  Updated result for submission ${result.submissionId}, test ${result.testCaseId}`
      );
    } catch (err) {
      console.error("❌ Error processing result message:", err);
    }
  }
}

runSubscriber().catch((err) => {
  console.error("Unhandled error in subscriber:", err);
  process.exit(1);
});
