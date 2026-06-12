(function () {
  var showButton = document.querySelector("[data-show-topics]");
  var list = document.querySelector("[data-topics-list]");
  var count = document.querySelector("[data-topics-count]");
  var status = document.querySelector("[data-topics-status]");
  var empty = document.querySelector("[data-topics-empty]");

  if (!showButton || !list || !count || !status) {
    return;
  }

  function topicValue(topic, key) {
    return topic[key] || topic[key.charAt(0).toUpperCase() + key.slice(1)] || "";
  }

  function setCount(total) {
    count.textContent = total + " total";
  }

  function reloadAfterAction(response) {
    if (!response.ok) {
      throw new Error("HTTP " + response.status);
    }
    window.location.reload();
  }

  function renderTopics(topics) {
    list.textContent = "";

    if (empty) {
      empty.hidden = topics.length > 0;
    }

    topics.forEach(function (topic) {
      var slug = topicValue(topic, "slug");
      var item = document.createElement("li");
      var meta = document.createElement("div");
      var name = document.createElement("strong");
      var slugText = document.createElement("span");
      var descriptionForm = document.createElement("form");
      var descriptionInput = document.createElement("textarea");
      var saveButton = document.createElement("button");
      var deleteButton = document.createElement("button");
      var description = topicValue(topic, "description");

      item.className = "topic-row";
      meta.className = "topic-meta";
      name.textContent = topicValue(topic, "name");
      slugText.textContent = slug;

      meta.appendChild(name);
      meta.appendChild(slugText);

      descriptionForm.className = "topic-description-form";
      descriptionForm.action = "/topics/" + encodeURIComponent(slug) + "/description";
      descriptionForm.setAttribute("data-topic-description-form", "");
      descriptionInput.className = "compact-textarea";
      descriptionInput.name = "description";
      descriptionInput.value = description;
      descriptionInput.setAttribute("aria-label", "Description for " + name.textContent);
      saveButton.className = "secondary-button";
      saveButton.type = "submit";
      saveButton.textContent = "Save description";
      descriptionForm.appendChild(descriptionInput);
      descriptionForm.appendChild(saveButton);
      meta.appendChild(descriptionForm);

      deleteButton.className = "danger-button";
      deleteButton.type = "button";
      deleteButton.textContent = "Delete";
      deleteButton.setAttribute("data-delete-topic", "");
      deleteButton.setAttribute("data-delete-url", "/topics/" + encodeURIComponent(slug));

      item.appendChild(meta);
      item.appendChild(deleteButton);
      list.appendChild(item);
    });

    setCount(topics.length);
  }

  showButton.addEventListener("click", function () {
    showButton.disabled = true;
    showButton.textContent = "Loading...";
    status.textContent = "";

    fetch("/topics", {
      headers: {
        Accept: "application/json",
      },
    })
      .then(function (response) {
        if (!response.ok) {
          throw new Error("HTTP " + response.status);
        }
        return response.json();
      })
      .then(function (topics) {
        renderTopics(topics);
        status.textContent = "Loaded " + topics.length + " topics.";
      })
      .catch(function () {
        status.textContent = "Failed to load topics.";
      })
      .finally(function () {
        showButton.disabled = false;
        showButton.textContent = "Show all topics";
      });
  });

  document.addEventListener("submit", function (event) {
    var form = event.target.closest("[data-topic-description-form]");
    if (!form) {
      return;
    }

    event.preventDefault();

    fetch(form.action, {
      method: "PATCH",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
      },
      body: new URLSearchParams(new FormData(form)),
    })
      .then(reloadAfterAction)
      .catch(function () {
        status.textContent = "Failed to update topic description.";
      });
  });

  document.addEventListener("click", function (event) {
    var button = event.target.closest("[data-delete-topic]");
    if (!button) {
      return;
    }

    fetch(button.getAttribute("data-delete-url"), {
      method: "DELETE",
    })
      .then(reloadAfterAction)
      .catch(function () {
        status.textContent = "Failed to delete topic.";
      });
  });
})();
