module.exports = async ({ github, context, core }) => {
  const { LARK_WEBHOOK, CARD_BODY } = process.env;

  // Send card
  await fetch(LARK_WEBHOOK, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      "msg_type": "interactive",
      "card": CARD_BODY
    }),
  }).then(async response => {
    if (!response.ok) {
      throw new Error(`HTTP error! Status: ${response.status}. Message: ${await response.text()}`);
    }
    return response.json();
  });
};