const timeout = AbortSignal.timeout(3000);

try {
  const response = await fetch("http://127.0.0.1:3000/health/ready", {
    signal: timeout,
  });
  if (!response.ok) process.exit(1);
} catch {
  process.exit(1);
}
