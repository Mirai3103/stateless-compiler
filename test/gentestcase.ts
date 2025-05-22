// generateTestCases.ts

import { randomUUID } from "crypto";

interface TestCase {
  id: string;
  input: string;
  expectOutput: string;
}

// Hàm tính Fibonacci chuẩn (dạng vòng lặp)
function fibonacci(n: number): number {
  if (n <= 1) return n;
  let a = 0,
    b = 1;
  for (let i = 2; i <= n; i++) {
    [a, b] = [b, a + b];
  }
  return b;
}

// Hàm sinh mảng test case
function generateTestCases(count: number): TestCase[] {
  const testCases: TestCase[] = [];
  for (let i = 0; i < count; i++) {
    const n = Math.floor(Math.random() * 100); // n từ 0 đến 29
    const input = `${n}`;
    const expectOutput = `${fibonacci(n)}`;
    testCases.push({
      id: randomUUID(),
      input,
      expectOutput,
    });
  }
  return testCases;
}

// Xuất ra console hoặc lưu file JSON
const testCases = generateTestCases(10);
Bun.write("testCases.json", JSON.stringify(testCases, null, 2));
