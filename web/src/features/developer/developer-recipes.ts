import type { DeveloperExample } from "@/lib/api/developer";

export type DeveloperRecipe = Readonly<{
  id: "curl" | "typescript" | "go" | "postman";
  label: string;
  summary: string;
  code: string;
}>;

function transferBody(example: DeveloperExample) {
  return JSON.stringify(example.body, null, 2);
}

export function buildTransferRecipes(example: DeveloperExample): DeveloperRecipe[] {
  const body = transferBody(example);
  const compactBody = JSON.stringify(example.body);

  return [
    {
      id: "curl",
      label: "curl",
      summary: "Use protected environment variables; the copied command contains no working credential.",
      code: `curl --fail-with-body --request ${example.method} "\${LEDGERSYNC_API_URL}${example.path}" \\
  --header "Authorization: Bearer \${LEDGERSYNC_ACCESS_TOKEN}" \\
  --header "Content-Type: ${example.headers["Content-Type"]}" \\
  --header "Idempotency-Key: ${example.headers["Idempotency-Key"]}" \\
  --data '${compactBody}'`,
    },
    {
      id: "typescript",
      label: "TypeScript",
      summary: "Keep money as a string and persist the key with the request body before the first send.",
      code: `const idempotencyKey = "${example.headers["Idempotency-Key"]}";
const body = ${body};

const response = await fetch(baseUrl + "${example.path}", {
  method: "${example.method}",
  headers: {
    Authorization: \`Bearer \${accessToken}\`,
    "Content-Type": "${example.headers["Content-Type"]}",
    "Idempotency-Key": idempotencyKey
  },
  body: JSON.stringify(body)
});

// On a timeout or lost response, resend this exact body with this exact key.`,
    },
    {
      id: "go",
      label: "Go",
      summary: "Build the body from exact strings and reuse the persisted key after an unknown response.",
      code: `body := []byte(${JSON.stringify(compactBody)})
request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"${example.path}", bytes.NewReader(body))
if err != nil { return err }
request.Header.Set("Authorization", "Bearer "+accessToken)
request.Header.Set("Content-Type", "${example.headers["Content-Type"]}")
request.Header.Set("Idempotency-Key", "${example.headers["Idempotency-Key"]}")

response, err := client.Do(request)
// If the send outcome is unknown, submit the identical body and key again.`,
    },
    {
      id: "postman",
      label: "Postman",
      summary: "Import the generated catalogue, then set secrets only in a private local environment.",
      code: `Collection: contracts/generated/ledgersync.postman_collection.json
Variables to set privately: baseUrl, bearerToken, actorAssertion
Operation: ${example.operation_id}
Schema: ${example.request_schema}

The generated collection contains placeholders only and must never be committed with environment values.`,
    },
  ];
}
