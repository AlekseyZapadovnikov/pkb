(function () {
  var messageForm = document.querySelector("[data-message-form]");
  var messageStatus = document.querySelector("[data-message-status]");
  var messagePending = false;
  var topicForm = document.querySelector("[data-topic-form]");
  var createButton = document.querySelector("[data-create-topic]");
  var closeButton = document.querySelector("[data-close-topic]");
  var showButton = document.querySelector("[data-show-topics]");
  var list = document.querySelector("[data-topics-list]");
  var count = document.querySelector("[data-topics-count]");
  var status = document.querySelector("[data-topics-status]");
  var empty = document.querySelector("[data-topics-empty]");
  var jobsList = document.querySelector("[data-jobs-list]");
  var jobsEmpty = document.querySelector("[data-jobs-empty]");

  function jobValue(job, key) {
    return job[key] || job[key.charAt(0).toUpperCase() + key.slice(1)] || "";
  }

  function setJobRow(row, job) {
    var statusText = row.querySelector("[data-job-status]");
    var errorText = row.querySelector("[data-job-error]");
    var updatedText = row.querySelector("[data-job-updated]");
    var error = jobValue(job, "error");
    var labels = { pending: "Waiting", running: "Processing", done: "Completed", failed: "Failed" };

    if (statusText) {
      statusText.textContent = labels[jobValue(job, "status")] || "Unknown status";
    }
    if (errorText) {
      errorText.textContent = error || "";
      errorText.hidden = !error;
    }
    if (updatedText) {
      var updated = jobValue(job, "updated_at");
      updatedText.textContent = updated ? "Updated " + new Date(updated).toLocaleString() : "";
    }
  }

  function renderJob(job, rawText) {
    var id;
    var row;
    var meta;
    var title;
    var statusText;
    var jobIDText;
    var updatedText;
    var errorText;
    var actions;
    var updateButton;

    if (!jobsList) {
      return;
    }

    id = jobValue(job, "job_id") || jobValue(job, "id");
    if (!id) {
      return;
    }

    row = jobsList.querySelector('[data-job-id="' + id + '"]');
    if (!row) {
      row = document.createElement("li");
      meta = document.createElement("div");
      title = document.createElement("strong");
      statusText = document.createElement("span");
      updatedText = document.createElement("span");
      errorText = document.createElement("p");
      actions = document.createElement("div");
      updateButton = document.createElement("button");

      row.className = "job-row";
      row.setAttribute("data-job-id", id);
      meta.className = "job-meta";
      var excerpt = Array.from((rawText || "Note").replace(/\s+/g, " ").trim());
      title.textContent = excerpt.slice(0, 90).join("") + (excerpt.length > 90 ? "…" : "");
      jobIDText = document.createElement("span");
      jobIDText.className = "job-updated";
      jobIDText.textContent = "Job #" + id;
      statusText.className = "job-state";
      statusText.setAttribute("data-job-status", "");
      updatedText.className = "job-updated";
      updatedText.setAttribute("data-job-updated", "");
      errorText.className = "job-error";
      errorText.setAttribute("data-job-error", "");
      errorText.hidden = true;
      actions.className = "job-row-actions";
      updateButton.className = "secondary-button";
      updateButton.type = "button";
      updateButton.textContent = "Refresh status";
      updateButton.setAttribute("data-update-job", "");
      updateButton.setAttribute("data-job-url", "/jobs/" + encodeURIComponent(id));

      meta.appendChild(title);
      meta.appendChild(statusText);
      meta.appendChild(jobIDText);
      meta.appendChild(updatedText);
      meta.appendChild(errorText);
      actions.appendChild(updateButton);
      row.appendChild(meta);
      row.appendChild(actions);
      jobsList.prepend(row);
    }

    setJobRow(row, job);
    if (jobsEmpty) {
      jobsEmpty.hidden = true;
    }
  }

  function updateJob(button) {
    var row = button.closest("[data-job-id]");

    if (!row) {
      return;
    }

    button.disabled = true;
    fetch(button.getAttribute("data-job-url"), {
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
      .then(function (job) {
        renderJob(job);
      })
      .catch(function () {
        var errorText = row.querySelector("[data-job-error]");
        if (errorText) {
          errorText.textContent = "Failed to update job state.";
          errorText.hidden = false;
        }
      })
      .finally(function () {
        button.disabled = false;
      });
  }

  if (messageForm && messageStatus) {
    messageForm.addEventListener("submit", function (event) {
      var submitButton = messageForm.querySelector("button[type='submit']");

      event.preventDefault();
      if (messagePending) {
        return;
      }
      messagePending = true;
      var rawText = messageForm.elements.raw_text.value;
      messageStatus.textContent = "Saving…";
      if (submitButton) {
        submitButton.disabled = true;
      }

      fetch(messageForm.action, {
        method: "POST",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
          Accept: "application/json",
        },
        body: new URLSearchParams(new FormData(messageForm)),
      })
        .then(function (response) {
          if (!response.ok) {
            throw new Error("HTTP " + response.status);
          }
          return response.json();
        })
        .then(function (job) {
          if (messageForm.elements.raw_text.value === rawText) {
            messageForm.reset();
          }
          renderJob(job, rawText);
          messageStatus.textContent = "Note saved. Refresh its status below to check processing.";
        })
        .catch(function () {
          messageStatus.textContent = "Failed to add note.";
        })
        .finally(function () {
          messagePending = false;
          if (submitButton) {
            submitButton.disabled = false;
          }
        });
    });
  }

  function topicValue(topic, key) {
    return topic[key] || topic[key.charAt(0).toUpperCase() + key.slice(1)] || "";
  }

  function setCount(total) {
    count.textContent = total + " total";
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
      var descriptionText = document.createElement("p");
      var descriptionForm = document.createElement("form");
      var descriptionInput = document.createElement("textarea");
      var saveButton = document.createElement("button");
      var cancelButton = document.createElement("button");
      var editActions = document.createElement("div");
      var rowActions = document.createElement("div");
      var updateButton = document.createElement("button");
      var deleteButton = document.createElement("button");
      var description = topicValue(topic, "description");

      item.className = "topic-row";
      meta.className = "topic-meta";
      name.textContent = topicValue(topic, "name");
      slugText.textContent = slug;
      descriptionText.textContent = description || "No description.";
      descriptionText.setAttribute("data-topic-description", "");

      meta.appendChild(name);
      meta.appendChild(slugText);
      meta.appendChild(descriptionText);

      descriptionForm.className = "topic-description-form";
      descriptionForm.action = "/topics/" + encodeURIComponent(slug) + "/description";
      descriptionForm.setAttribute("data-topic-description-form", "");
      descriptionForm.hidden = true;
      descriptionInput.className = "compact-textarea";
      descriptionInput.name = "description";
      descriptionInput.value = description;
      descriptionInput.defaultValue = description;
      descriptionInput.setAttribute("aria-label", "Description for " + name.textContent);
      editActions.className = "topic-edit-actions";
      saveButton.className = "secondary-button";
      saveButton.type = "submit";
      saveButton.textContent = "Save";
      cancelButton.className = "secondary-button";
      cancelButton.type = "button";
      cancelButton.textContent = "Cancel";
      cancelButton.setAttribute("data-cancel-topic-update", "");
      descriptionForm.appendChild(descriptionInput);
      editActions.appendChild(saveButton);
      editActions.appendChild(cancelButton);
      descriptionForm.appendChild(editActions);
      meta.appendChild(descriptionForm);

      rowActions.className = "topic-row-actions";
      updateButton.className = "secondary-button";
      updateButton.type = "button";
      updateButton.textContent = "Edit description";
      updateButton.setAttribute("data-update-topic", "");
      deleteButton.className = "danger-button";
      deleteButton.type = "button";
      deleteButton.textContent = "Delete";
      deleteButton.setAttribute("data-delete-topic", "");
      deleteButton.setAttribute("data-delete-url", "/topics/" + encodeURIComponent(slug));
      rowActions.appendChild(updateButton);
      rowActions.appendChild(deleteButton);

      item.appendChild(meta);
      item.appendChild(rowActions);
      list.appendChild(item);
    });

    setCount(topics.length);
  }

  function closeTopicEditor(row) {
    var form = row.querySelector("[data-topic-description-form]");
    var description = row.querySelector("[data-topic-description]");
    var updateButton = row.querySelector("[data-update-topic]");

    if (form) {
      form.reset();
      form.hidden = true;
    }
    if (description) {
      description.hidden = false;
    }
    if (updateButton) {
      updateButton.hidden = false;
      updateButton.focus();
    }
  }

  function loadTopics(message) {
    return fetch("/topics", { headers: { Accept: "application/json" } })
      .then(function (response) {
        if (!response.ok) {
          throw new Error("HTTP " + response.status);
        }
        return response.json();
      })
      .then(function (topics) {
        renderTopics(topics || []);
        status.textContent = message;
      });
  }

  function closeCreateForm() {
    topicForm.hidden = true;
    createButton.setAttribute("aria-expanded", "false");
    createButton.focus();
  }

  if (topicForm && createButton && closeButton && status) {
    createButton.addEventListener("click", function () {
      topicForm.hidden = !topicForm.hidden;
      createButton.setAttribute("aria-expanded", String(!topicForm.hidden));
      if (!topicForm.hidden) {
        topicForm.elements.name.focus();
      }
    });
    closeButton.addEventListener("click", closeCreateForm);
    topicForm.addEventListener("submit", function (event) {
      event.preventDefault();
      var button = topicForm.querySelector("button[type='submit']");
      if (button.disabled) {
        return;
      }
      button.disabled = true;
      status.textContent = "Creating topic…";
      fetch(topicForm.action, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: new URLSearchParams(new FormData(topicForm)),
      })
        .then(function (response) {
          if (!response.ok) {
            throw new Error("HTTP " + response.status);
          }
          topicForm.reset();
          closeCreateForm();
          return loadTopics("Topic created.");
        })
        .catch(function () {
          status.textContent = "Failed to create or reload topic. Refresh topics to check before trying again.";
        })
        .finally(function () {
          button.disabled = false;
        });
    });
  }

  if (showButton && list && count && status) {
    showButton.addEventListener("click", function () {
      showButton.disabled = true;
      showButton.textContent = "Loading...";
      status.textContent = "";

      loadTopics("Topics refreshed.")
        .catch(function () {
          status.textContent = "Failed to load topics.";
        })
        .finally(function () {
          showButton.disabled = false;
          showButton.textContent = "Refresh topics";
        });
    });
  }

  document.addEventListener("submit", function (event) {
    var form = event.target.closest("[data-topic-description-form]");
    if (!form || !status) {
      return;
    }

    event.preventDefault();
    var button = form.querySelector("button[type='submit']");
    if (button.disabled) {
      return;
    }
    button.disabled = true;
    status.textContent = "Saving description…";

    fetch(form.action, {
      method: "PATCH",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
      },
      body: new URLSearchParams(new FormData(form)),
    })
      .then(function (response) {
        if (!response.ok) {
          throw new Error("HTTP " + response.status);
        }
        return loadTopics("Description updated.");
      })
      .then(function () {
        createButton.focus();
      })
      .catch(function () {
        status.textContent = "Failed to update or reload description. Refresh topics to check before trying again.";
      })
      .finally(function () {
        button.disabled = false;
      });
  });

  document.addEventListener("click", function (event) {
    var updateJobButton = event.target.closest("[data-update-job]");
    var updateButton = event.target.closest("[data-update-topic]");
    var cancelButton = event.target.closest("[data-cancel-topic-update]");
    var deleteButton = event.target.closest("[data-delete-topic]");
    var row;
    var form;
    var description;
    var input;

    if (updateJobButton) {
      updateJob(updateJobButton);
      return;
    }

    if (updateButton) {
      row = updateButton.closest(".topic-row");
      form = row.querySelector("[data-topic-description-form]");
      description = row.querySelector("[data-topic-description]");
      input = form.querySelector("textarea");

      form.hidden = false;
      if (description) {
        description.hidden = true;
      }
      updateButton.hidden = true;
      if (input) {
        input.focus();
      }
      return;
    }

    if (cancelButton) {
      row = cancelButton.closest(".topic-row");
      closeTopicEditor(row);
      return;
    }

    if (!deleteButton || !status || deleteButton.disabled) {
      return;
    }

    deleteButton.disabled = true;
    status.textContent = "Deleting topic…";
    fetch(deleteButton.getAttribute("data-delete-url"), {
      method: "DELETE",
    })
      .then(function (response) {
        if (!response.ok) {
          throw new Error("HTTP " + response.status);
        }
        return loadTopics("Topic deleted.");
      })
      .then(function () {
        createButton.focus();
      })
      .catch(function () {
        status.textContent = "Failed to delete or reload topic. Refresh topics to check before trying again.";
      })
      .finally(function () {
        deleteButton.disabled = false;
      });
  });
})();
