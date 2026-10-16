# Install Hallido on a QNAP NAS

For QTS 5 / QuTS hero 5 with Container Station 3. Takes about 10 minutes.

You need the file **`compose.yaml`** from
[the latest release](https://github.com/ohm-meter/hallido/releases/latest)
on your computer ([what is that?](README.md#the-three-steps)).

> The pictures are placeholders and will be replaced by real screenshots.

## Step 1: Install Container Station

1. Log in to your QNAP in the browser as an administrator.
2. Open **App Center**.
3. Search for **Container Station** and click **Install**.
   If it is already installed, go to step 2. On the first start, Container
   Station asks for a folder for its data – accept the suggestion.

![App Center with Container Station](../img/qnap-01-app-center.svg)

## Step 2: Create the Hallido application

1. Open **Container Station**.
2. Click **Applications** on the left, then **Create**.

   ![Container Station: Applications → Create](../img/qnap-02-create-application.svg)

3. Fill in the form:
   - **Application name:** `hallido`
   - Open the downloaded file `compose.yaml` with a text editor (on Windows
     “Editor”/Notepad, on a Mac “TextEdit”), select everything, copy it and
     paste it into the large YAML field. (Some versions also offer
     **Upload**.)
4. Click **Validate**. When it says the YAML is valid, click **Create**.

   ![Name, YAML and Create](../img/qnap-03-paste-yaml.svg)

Container Station now downloads Hallido (about 12 MB) and starts it. After a
moment, the application `hallido` is shown as **Running**.

![Application is running](../img/qnap-04-running.svg)

## Step 3: Find the address of your NAS

You probably already know it – it is the address you use to log in to the
QNAP, without `:8080`. Otherwise: **Control Panel → Network & Virtual
Switch → Interfaces**. The address looks like `192.168.1.20`.

![Network & Virtual Switch](../img/qnap-05-ip-address.svg)

## Step 4: Set up Hallido in the browser

Do this **within one hour** after Hallido started (for your security). If
the time is over, restart the application: Container Station →
**Applications** → `hallido` → **Restart**.

1. On a phone or computer **in your home network**, open
   `http://192.168.1.20:8790` – with the address of your NAS instead of
   `192.168.1.20`. Use `http://` and port **8790** (not 8080, that is the
   QNAP itself).
2. Follow the four steps in the browser: **Create access key** → save the
   key (it is shown only once) → **Find speakers** → **Create first scene**
   or **Later**. The pictures are in the
   [Synology guide](synology.md#set-up-hallido); they are the same on QNAP.

**On another phone or computer:** open the same address, go to the tab
**Speakers → Change API token** and enter your access key.

Something not working? See [If something doesn't work](troubleshooting.md).
