(function () {
  document.querySelectorAll("[data-knowledge-id]").forEach(function (card) {
    var form = card.querySelector(".knowledge-edit-form");
    var editButton = card.querySelector("[data-edit-knowledge]");
    var saveButton = form.querySelector('[type="submit"]');
    var cancelButton = form.querySelector("[data-cancel-edit]");
    var clearButton = form.querySelector("[data-clear-topics]");
    var error = form.querySelector("[data-edit-error]");
    var jsonError = form.querySelector("[data-json-error]");
    var pending = false;
    var topicsReady = false;
    var url = "/api/knowledge/" + encodeURIComponent(card.dataset.knowledgeId);

    async function readResponse(response) {
      if (!response.ok) {
        throw new Error((await response.text()).trim() || "HTTP " + response.status);
      }
      return response;
    }

    editButton.addEventListener("click", async function () {
      editButton.disabled = true;
      form.hidden = false;
      saveButton.disabled = true;
      error.hidden = true;
      topicsReady = false;
      try {
        var response = await readResponse(await fetch("/topics", { headers: { Accept: "application/json" } }));
        var topics = await response.json();
        var selected = new Set(Array.from(card.querySelectorAll("[data-topic-id]"), function (badge) {
          return badge.dataset.topicId;
        }));
        form.elements.topics.replaceChildren();
        topics.forEach(function (topic) {
          form.elements.topics.add(new Option(topic.Name, String(topic.ID), false, selected.has(String(topic.ID))));
        });
        topicsReady = true;
        saveButton.disabled = false;
        form.elements.title.focus();
      } catch (err) {
        error.textContent = "Failed to load topics: " + err.message;
        error.hidden = false;
        editButton.disabled = false;
      }
    });

    cancelButton.addEventListener("click", function () {
      if (pending) return;
      form.reset();
      form.hidden = true;
      editButton.disabled = false;
      jsonError.hidden = true;
    });

    clearButton.addEventListener("click", function () {
      Array.from(form.elements.topics.options).forEach(function (option) { option.selected = false; });
    });

    form.addEventListener("submit", async function (event) {
      event.preventDefault();
      if (pending || !topicsReady) return;
      error.hidden = true;
      jsonError.hidden = true;
      if (!form.elements.title.value.trim() || !form.elements.body.value.trim()) {
        error.textContent = "Title and body must not be empty.";
        error.hidden = false;
        return;
      }

      var dataJSON = null;
      var rawJSON = form.elements.data_json.value.trim();
      if (rawJSON) {
        try {
          dataJSON = JSON.parse(rawJSON);
        } catch (err) {
          jsonError.textContent = "Invalid JSON: " + err.message;
          jsonError.hidden = false;
          form.elements.data_json.focus();
          return;
        }
      }

      pending = true;
      saveButton.disabled = true;
      cancelButton.disabled = true;
      clearButton.disabled = true;
      var saved = false;
      try {
        await readResponse(await fetch(url, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            title: form.elements.title.value,
            body: form.elements.body.value,
            topics: Array.from(form.elements.topics.selectedOptions, function (option) { return Number(option.value); }),
            data_json: dataJSON,
          }),
        }));
        saved = true;
        var response = await readResponse(await fetch(url, { headers: { Accept: "application/json" } }));
        var item = await response.json();
        card.querySelector("h2").textContent = item.Body.Title;
        card.querySelector(".knowledge-body").textContent = item.Body.Body;
        var badges = card.querySelector(".topic-badges");
        badges.replaceChildren();
        (item.Body.Topics.length ? item.Body.Topics : [{ Name: "Без темы" }]).forEach(function (topic) {
          var badge = document.createElement("span");
          badge.className = "topic-badge";
          badge.textContent = topic.Name;
          if (topic.ID) badge.dataset.topicId = String(topic.ID);
          badges.appendChild(badge);
        });
        var details = card.querySelector("[data-knowledge-json]");
        details.hidden = item.Body.DataJSON === null;
        var formattedJSON = details.hidden ? "" : JSON.stringify(item.Body.DataJSON, null, 2);
        details.querySelector("pre").textContent = formattedJSON;
        form.elements.title.value = form.elements.title.defaultValue = item.Body.Title;
        form.elements.body.value = form.elements.body.defaultValue = item.Body.Body;
        form.elements.data_json.value = form.elements.data_json.defaultValue = formattedJSON;
        form.hidden = true;
        editButton.disabled = false;
      } catch (err) {
        error.textContent = (saved ? "Saved, but failed to refresh the card: " : "Failed to save: ") + err.message;
        error.hidden = false;
      } finally {
        pending = false;
        saveButton.disabled = false;
        cancelButton.disabled = false;
        clearButton.disabled = false;
      }
    });
  });
})();
