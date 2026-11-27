import os
from locust import HttpUser, task, between


class SiriusUser(HttpUser):
    wait_time = between(0.01, 0.05)

    def on_start(self):
        self.client.headers.update({"Authorization": "Bearer " + os.environ["LOAD_TOKEN"]})

    @task(7)
    def menu(self):
        self.client.get("/menu/items")

    @task(2)
    def item(self):
        self.client.get("/menu/items/1")

    @task(1)
    def order(self):
        self.client.post("/orders/create", json={"item_id": 1, "quantity": 1})
