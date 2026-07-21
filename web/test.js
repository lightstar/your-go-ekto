const evidencePrefix = "/api/v1/evidence/";
const entitiesPrefix = "/api/v1/entities/";

const form = document.querySelector("form");
const submitButton = document.querySelector("#submit-button");
const resultBlock = document.querySelector("#result");
const statusBlock = document.querySelector("#status");
const resourcesBlock = document.querySelector("#resources");

const savedResources = document.querySelector("#saved-resources");
const dossierLink = document.querySelector("#dossier-link");
const savedEvidenceList = document.querySelector("#saved-evidence-list");

const failedResources = document.querySelector("#failed-resources");
const failedEvidenceList = document.querySelector("#failed-evidence-list");

form.addEventListener("submit", async (event) => {
  event.preventDefault();

  resultBlock.textContent = "Загрузка...";
  submitButton.disabled = true;
  resourcesBlock.hidden = true;

  savedEvidenceList.replaceChildren();
  failedEvidenceList.replaceChildren();

  try {
    const response = await fetch(form.action, {
      method: form.method,
      body: new FormData(form),
    });

    const data = await (async function () {
      const contentType = (response.headers.get("content-type") ?? "")
        .trim()
        .toLowerCase();

      if (contentType.split(";", 1)[0].trim() !== "application/json") {
        const error =
          "not json response: " + (await response.text()).trim().slice(0, 500);
        return { error: error };
      }

      const data = await response.json();

      if (data === null || typeof data !== "object" || Array.isArray(data)) {
        throw new Error("Сервер вернул ответ неожиданного формата");
      }

      return data;
    })();

    if (!response.ok || data.error) {
      resultBlock.textContent =
        "Ошибка загрузки: " + (data.error || `Ошибка HTTP ${response.status}`);
    } else {
      resultBlock.textContent = "Загрузка успешна";
    }

    const savedEvidence = data.saved_evidence ?? [];
    const failedEvidence = data.failed_evidence ?? [];

    if (!Array.isArray(savedEvidence) || !Array.isArray(failedEvidence)) {
      throw new Error("Сервер вернул ответ неожиданного формата");
    }

    statusBlock.hidden = data.status === undefined;
    resourcesBlock.hidden =
      data.dossier_id === undefined && failedEvidence.length === 0;
    savedResources.hidden = data.dossier_id === undefined;
    failedResources.hidden = failedEvidence.length === 0;

    if (data.status) {
      const responseStatus = document.querySelector("#status-text");
      responseStatus.textContent = `${data.status} (${response.status})`;
    }

    if (data.dossier_id) {
      dossierLink.href = entitiesPrefix + encodeURIComponent(data.dossier_id);
      dossierLink.textContent = `Досье ${data.dossier_id}`;
    }

    for (const evidence of savedEvidence) {
      const link = document.createElement("a");
      const item = document.createElement("li");

      const filename = evidence.startsWith(evidencePrefix)
        ? evidence.slice(evidencePrefix.length)
        : evidence;

      link.target = "_blank";
      link.href = evidence;
      link.textContent = `Улика ${filename}`;

      item.append(link);
      savedEvidenceList.append(item);
    }

    for (const evidence of failedEvidence) {
      const item = document.createElement("li");
      item.textContent = `${evidence.original_name}: ${evidence.reason}`;
      failedEvidenceList.append(item);
    }
  } catch (error) {
    resultBlock.textContent = `Ошибка загрузки: ${error.message}`;
  } finally {
    submitButton.disabled = false;
  }
});
