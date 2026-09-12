import React, { useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  BrowserRouter,
  Link,
  Navigate,
  Outlet,
  Route,
  Routes,
  useLocation,
  useNavigate,
  useParams,
} from "react-router-dom";
import {
  QueryClient,
  QueryClientProvider,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  ConfigProvider,
  Descriptions,
  Form,
  Input,
  Layout,
  Menu,
  Popconfirm,
  Row,
  Select,
  Space,
  Spin,
  Statistic,
  Table,
  Tag,
  Typography,
} from "antd";
import { api, Device, setCSRF } from "./api";
import "./style.css";
const client = new QueryClient({
  defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
});
const date = (v: number) => new Date(v * 1000).toLocaleString();
function ErrorBox({ error }: { error: unknown }) {
  return error ? (
    <Alert
      type="error"
      showIcon
      message={String(error instanceof Error ? error.message : error)}
    />
  ) : null;
}
function Login() {
  const nav = useNavigate();
  const [error, setError] = useState<unknown>();
  const [busy, setBusy] = useState(false);
  return (
    <div className="login">
      <Card title="rustdesk-control">
        <Typography.Paragraph type="secondary">
          Sign in to your device management workspace.
        </Typography.Paragraph>
        <ErrorBox error={error} />
        <Form
          layout="vertical"
          onFinish={async (v) => {
            setBusy(true);
            try {
              const s = await api("/login", "POST", v);
              setCSRF(s.csrf_token);
              client.clear();
              nav("/dashboard");
            } catch (e) {
              setError(e);
            } finally {
              setBusy(false);
            }
          }}
        >
          <Form.Item
            name="password"
            label="Administrator password"
            rules={[{ required: true }]}
          >
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Button type="primary" htmlType="submit" block loading={busy}>
            Sign in
          </Button>
        </Form>
      </Card>
    </div>
  );
}
function Shell() {
  const location = useLocation();
  const nav = useNavigate();
  const { message } = App.useApp();
  const session = useQuery({
    queryKey: ["session"],
    queryFn: () => api("/session"),
  });
  useEffect(() => {
    if (session.data) setCSRF(session.data.csrf_token);
  }, [session.data]);
  if (session.isPending) return <Spin fullscreen />;
  if (session.error) return <Navigate to="/login" replace />;
  return (
    <Layout className="shell">
      <Layout.Sider breakpoint="lg" collapsedWidth="0" width={230}>
        <div className="brand">
          rustdesk-control<small>DEVICE MANAGEMENT</small>
        </div>
        <Menu
          theme="dark"
          selectedKeys={["/" + location.pathname.split("/")[1]]}
          items={["dashboard", "devices", "approvals", "settings", "audit"].map(
            (p) => ({
              key: "/" + p,
              label: (
                <Link to={"/" + p}>{p[0].toUpperCase() + p.slice(1)}</Link>
              ),
            }),
          )}
        />
      </Layout.Sider>
      <Layout>
        <Layout.Header className="header">
          <Typography.Text>Organization workspace</Typography.Text>
          <Button
            onClick={async () => {
              try {
                await api("/logout", "POST");
                setCSRF("");
                client.clear();
                nav("/login");
              } catch (e) {
                message.error(String(e));
              }
            }}
          >
            Sign out
          </Button>
        </Layout.Header>
        <Layout.Content className="content">
          <Outlet />
        </Layout.Content>
      </Layout>
    </Layout>
  );
}
const columns = [
  {
    title: "RustDesk ID",
    dataIndex: "rustdesk_id",
    render: (v: string, d: Device) => (
      <Link to={"/devices/" + d.id}>{v || "Waiting for ID"}</Link>
    ),
  },
  { title: "Hostname", dataIndex: "hostname" },
  { title: "OS", dataIndex: "os" },
  { title: "RustDesk version", dataIndex: "rustdesk_version" },
  { title: "Managed client", dataIndex: "managed_client_version" },
  {
    title: "Status",
    dataIndex: "online",
    render: (v: boolean) => (
      <Tag color={v ? "green" : "default"}>{v ? "Online" : "Offline"}</Tag>
    ),
  },
  {
    title: "Approval",
    dataIndex: "status",
    render: (v: string) => (
      <Tag
        color={v === "approved" ? "blue" : v === "pending" ? "orange" : "red"}
      >
        {v}
      </Tag>
    ),
  },
  {
    title: "Password",
    render: (_: unknown, d: Device) =>
      d.status === "approved" ? (
        <Tag color={d.password_synced ? "green" : "orange"}>
          {d.password_synced ? "Synced" : "Pending"}
        </Tag>
      ) : (
        "—"
      ),
  },
  { title: "Last seen", dataIndex: "last_seen_at", render: date },
  {
    title: "Actions",
    render: (_: unknown, d: Device) => (
      <Link to={"/devices/" + d.id}>Details</Link>
    ),
  },
];
function Devices({
  approvals = false,
  recent = false,
}: {
  approvals?: boolean;
  recent?: boolean;
}) {
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [online, setOnline] = useState("");
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const q = new URLSearchParams({
    search,
    status: approvals ? "pending" : status,
    online,
    page: String(page),
    page_size: String(recent ? 5 : size),
  });
  const data = useQuery({
    queryKey: ["devices", q.toString()],
    queryFn: () => api("/devices?" + q),
    refetchInterval: 15000,
  });
  return (
    <Space direction="vertical" size="large" className="full">
      {!recent && (
        <div>
          <Typography.Title level={2}>
            {approvals ? "Pending approvals" : "Devices"}
          </Typography.Title>
          <Typography.Text type="secondary">
            {approvals
              ? "Review new installations before assigning managed access."
              : "Inventory and synchronization status. Updated every 15 seconds."}
          </Typography.Text>
        </div>
      )}
      {!recent && (
        <Space wrap>
          <Input.Search
            placeholder="Search ID or hostname"
            allowClear
            onSearch={(v) => {
              setSearch(v);
              setPage(1);
            }}
            style={{ width: 260 }}
          />
          <Select
            aria-label="Online status"
            value={online}
            onChange={(v) => {
              setOnline(v);
              setPage(1);
            }}
            options={[
              { value: "", label: "All connectivity" },
              { value: "true", label: "Online" },
              { value: "false", label: "Offline" },
            ]}
          />
          {!approvals && (
            <Select
              aria-label="Approval status"
              value={status}
              onChange={(v) => {
                setStatus(v);
                setPage(1);
              }}
              options={["", "pending", "approved", "rejected"].map((v) => ({
                value: v,
                label: v || "All approvals",
              }))}
            />
          )}
        </Space>
      )}
      <ErrorBox error={data.error} />
      <Table
        rowKey="id"
        loading={data.isPending}
        columns={columns}
        dataSource={data.data?.items}
        scroll={{ x: 1450 }}
        pagination={
          recent
            ? false
            : {
                current: page,
                pageSize: size,
                total: data.data?.total,
                showSizeChanger: true,
                onChange: (p, n) => {
                  setPage(p);
                  setSize(n);
                },
              }
        }
      />
    </Space>
  );
}
function Dashboard() {
  const q = useQuery({
    queryKey: ["dashboard"],
    queryFn: () => api("/dashboard"),
    refetchInterval: 15000,
  });
  return (
    <Space direction="vertical" size="large" className="full">
      <div>
        <Typography.Title level={2}>Dashboard</Typography.Title>
        <Typography.Text type="secondary">
          A clear view of your managed fleet.
        </Typography.Text>
      </div>
      <ErrorBox error={q.error} />
      <Row gutter={[16, 16]}>
        {[
          ["total", "Total devices"],
          ["online", "Online"],
          ["offline", "Offline"],
          ["pending", "Pending approval"],
          ["password_pending", "Password pending"],
        ].map(([k, v]) => (
          <Col xs={24} sm={12} xl={4} key={k}>
            <Card>
              <Statistic title={v} value={q.data?.[k] ?? "—"} />
            </Card>
          </Col>
        ))}
      </Row>
      <Card title="Recently seen devices">
        <Devices recent />
      </Card>
    </Space>
  );
}
function DetailRoute() {
  const { id } = useParams();
  return <Detail key={id} />;
}
function Detail() {
  const { id } = useParams();
  const active = useRef(true);
  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
    };
  }, []);
  const nav = useNavigate();
  const qc = useQueryClient();
  const { message } = App.useApp();
  const [secret, setSecret] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    setSecret("");
    return () => setSecret("");
  }, [id]);
  const q = useQuery<Device>({
    queryKey: ["device", id],
    queryFn: () => api("/devices/" + id),
    refetchInterval: 15000,
  });
  const settings = useQuery({
    queryKey: ["settings"],
    queryFn: () => api("/settings/rustdesk"),
    refetchInterval: 15000,
  });
  useEffect(() => {
    setSecret("");
  }, [q.data?.password_version, q.data?.status]);
  async function act(action: string) {
    setBusy(true);
    setSecret("");
    try {
      await api(
        "/devices/" + id + (action === "delete" ? "" : "/" + action),
        action === "delete" ? "DELETE" : "POST",
      );
      message.success("Device updated");
      qc.invalidateQueries();
      if (action === "delete") nav("/devices");
    } catch (e) {
      message.error(String(e));
    } finally {
      setBusy(false);
    }
  }
  if (q.isPending) return <Spin />;
  if (!q.data) return <ErrorBox error={q.error} />;
  const d = q.data;
  return (
    <Space direction="vertical" size="large" className="full">
      <Link to="/devices">← Devices</Link>
      <Typography.Title level={2}>{d.hostname}</Typography.Title>
      <ErrorBox error={q.error} />
      <Space wrap>
        {[
          "approve",
          "reject",
          "rotate-password",
          "reset-identity",
          "delete",
        ].map((a) => (
          <Popconfirm
            key={a}
            title={"Confirm " + a.replaceAll("-", " ") + "?"}
            description={
              a === "reset-identity"
                ? "The next signed enrollment can claim this UUID and will require approval."
                : a === "reject"
                  ? "Stops future credential delivery. The existing offline password remains on the device."
                  : "This action changes the managed device."
            }
            onConfirm={() => act(a)}
          >
            <Button
              danger={["delete", "reset-identity", "reject"].includes(a)}
              disabled={
                busy ||
                (a === "rotate-password" && d.status !== "approved") ||
                (a === "approve" && d.status === "approved")
              }
            >
              {a
                .split("-")
                .map((s) => s[0].toUpperCase() + s.slice(1))
                .join(" ")}
            </Button>
          </Popconfirm>
        ))}
      </Space>
      <Card>
        <Descriptions
          bordered
          column={{ xs: 1, sm: 1, md: 2, xl: 3 }}
          items={Object.entries(d)
            .filter(([k]) => k !== "id")
            .map(([k, v]) => ({
              key: k,
              label: k.replaceAll("_", " "),
              children: k.endsWith("_at") ? date(v as number) : String(v),
            }))}
        />
      </Card>
      <Card title="Managed access">
        <Space direction="vertical">
          <Typography.Text>
            Policy:{" "}
            {d.applied_policy_version === settings.data?.policy_version
              ? "Synced"
              : "Pending"}{" "}
            · Password {d.password_synced ? "synced" : "pending"}
          </Typography.Text>
          <Typography.Text copyable={{ text: d.rustdesk_id }}>
            RustDesk ID: {d.rustdesk_id}
          </Typography.Text>
          <Typography.Text code>{secret || "••••••••"}</Typography.Text>
          <Space wrap>
            <Button
              disabled={d.status !== "approved"}
              loading={busy}
              onClick={async () => {
                setBusy(true);
                try {
                  const data = await api("/devices/" + id + "/credential");
                  if (active.current) setSecret(data.password);
                } catch (e) {
                  message.error(String(e));
                } finally {
                  setBusy(false);
                }
              }}
            >
              Reveal Password
            </Button>
            <Button
              disabled={!secret}
              onClick={async () => {
                try {
                  await navigator.clipboard.writeText(secret);
                  message.success("Password copied");
                } catch {
                  message.error("Clipboard access unavailable");
                }
              }}
            >
              Copy Password
            </Button>
            {secret && <Button onClick={() => setSecret("")}>Hide</Button>}
          </Space>
          <Typography.Text type="secondary">
            Connect using a normal RustDesk client. Credentials are never
            included in connection URLs.
          </Typography.Text>
        </Space>
      </Card>
    </Space>
  );
}
function Settings() {
  const q = useQuery({
    queryKey: ["settings"],
    queryFn: () => api("/settings/rustdesk"),
  });
  const [form] = Form.useForm();
  const [busy, setBusy] = useState(false);
  const { message } = App.useApp();
  const nav = useNavigate();
  useEffect(() => {
    if (q.data) form.setFieldsValue(q.data.rustdesk);
  }, [q.data, form]);
  return (
    <Space direction="vertical" size="large" className="full">
      <Typography.Title level={2}>Settings</Typography.Title>
      <ErrorBox error={q.error} />
      <Card
        title={
          "RustDesk OSS server · Policy " + (q.data?.policy_version ?? "—")
        }
      >
        <Alert
          type="info"
          showIcon
          message="This configuration will be distributed to all managed devices."
        />
        <Form
          form={form}
          layout="vertical"
          onFinish={async (v) => {
            setBusy(true);
            try {
              await api("/settings/rustdesk", "PUT", { ...v, api_server: "" });
              client.invalidateQueries({ queryKey: ["settings"] });
              message.success("Configuration saved");
            } catch (e) {
              message.error(String(e));
            } finally {
              setBusy(false);
            }
          }}
        >
          <Form.Item
            name="id_server"
            label="ID Server"
            rules={[{ required: true }]}
          >
            <Input placeholder="rustdesk.example.com" />
          </Form.Item>
          <Form.Item name="relay_server" label="Relay Server">
            <Input placeholder="rustdesk.example.com" />
          </Form.Item>
          <Form.Item
            name="key"
            label="RustDesk Public Key"
            rules={[{ required: true }]}
          >
            <Input />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={busy}>
            Save configuration
          </Button>
        </Form>
      </Card>
      <Card title="Change administrator password">
        <Form
          layout="vertical"
          onFinish={async (v) => {
            try {
              await api("/password", "PUT", v);
              client.clear();
              setCSRF("");
              message.success("Password changed. Sign in again.");
              nav("/login");
            } catch (e) {
              message.error(String(e));
            }
          }}
        >
          <Form.Item
            name="current_password"
            label="Current password"
            rules={[{ required: true }]}
          >
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Form.Item
            name="new_password"
            label="New password"
            rules={[{ required: true, min: 12 }]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Button htmlType="submit">
            Change password and end all sessions
          </Button>
        </Form>
      </Card>
    </Space>
  );
}
function Audit() {
  const [page, setPage] = useState(1);
  const q = useQuery({
    queryKey: ["audit", page],
    queryFn: () => api("/audit?page=" + page),
    refetchInterval: 15000,
  });
  return (
    <Space direction="vertical" size="large" className="full">
      <Typography.Title level={2}>Audit log</Typography.Title>
      <ErrorBox error={q.error} />
      <Table
        rowKey="id"
        loading={q.isPending}
        dataSource={q.data?.items}
        scroll={{ x: 700 }}
        columns={[
          { title: "Time", dataIndex: "timestamp", render: date },
          { title: "Administrator", dataIndex: "admin" },
          { title: "Action", dataIndex: "action" },
          { title: "Device", dataIndex: "device" },
        ]}
        pagination={{
          current: page,
          pageSize: 20,
          total: q.data?.total,
          onChange: setPage,
          showSizeChanger: false,
        }}
      />
    </Space>
  );
}
createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ConfigProvider
      theme={{
        token: {
          colorPrimary: "#176f86",
          borderRadius: 8,
          fontFamily: "Inter, system-ui, sans-serif",
        },
        components: {
          Layout: { siderBg: "#102b40" },
          Menu: { darkItemBg: "#102b40" },
        },
      }}
    >
      <App>
        <QueryClientProvider client={client}>
          <BrowserRouter>
            <Routes>
              <Route path="/login" element={<Login />} />
              <Route element={<Shell />}>
                <Route path="/dashboard" element={<Dashboard />} />
                <Route path="/devices" element={<Devices />} />
                <Route path="/devices/:id" element={<DetailRoute />} />
                <Route path="/approvals" element={<Devices approvals />} />
                <Route path="/settings" element={<Settings />} />
                <Route path="/audit" element={<Audit />} />
                <Route
                  path="*"
                  element={<Navigate to="/dashboard" replace />}
                />
              </Route>
            </Routes>
          </BrowserRouter>
        </QueryClientProvider>
      </App>
    </ConfigProvider>
  </React.StrictMode>,
);
