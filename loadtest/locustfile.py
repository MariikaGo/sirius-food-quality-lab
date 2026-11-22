from locust import HttpUser,task,between
class MenuUser(HttpUser):
    wait_time=between(0.1,0.5)
    @task
    def menu(self): self.client.get("/menu/items",name="menu")
