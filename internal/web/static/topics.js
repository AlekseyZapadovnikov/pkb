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
      var form = document.createElement("form");
      var button = document.createElement("button");
      var description = topicValue(topic, "description");

      item.className = "topic-row";
      meta.className = "topic-meta";
      name.textContent = topicValue(topic, "name");
      slugText.textContent = slug;

      meta.appendChild(name);
      meta.appendChild(slugText);

      if (description) {
        var descriptionText = document.createElement("p");
        descriptionText.textContent = description;
        meta.appendChild(descriptionText);
      }

      form.action = "/topics/" + encodeURIComponent(slug) + "/delete";
      form.method = "post";
      button.className = "danger-button";
      button.type = "submit";
      button.textContent = "Delete";
      form.appendChild(button);

      item.appendChild(meta);
      item.appendChild(form);
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
})();
