var sandbox = require("../testSandbox");

var expect = require("expect.js");
var $ = require("jquery");
var React = require("react");
var ReactDOM = require("react-dom");
var TestUtils = require("react-addons-test-utils");
var RepoBuildIndicator = require("./RepoBuildIndicator");
var client = require("../client");

describe("RepoBuildIndicator", () => {
	// Render function tests. The object key is also the expected BuildStatus.
	var renderTests = {
		FAILURE: {
			Failure: true,
			EndedAt: "2014-12-20 22:53:11",
			CommitID: "CommID123",
			ID: 1,
			expect: {
				cls: "danger",
				txt: "failed",
				icon: "fa-exclamation-circle",
			},
		},
		BUILT: {
			Success: true,
			EndedAt: "2014-12-20 22:53:11",
			ID: 1,
			CommitID: "CSmmID123",
			expect: {
				cls: "success",
				txt: "succeeded",
				icon: "fa-check",
			},
		},
		STARTED: {
			StartedAt: "2014-12-20 22:53:11",
			CommitID: "CTmmID123",
			ID: 1,
			expect: {
				cls: "primary",
				txt: "started",
				icon: "fa-circle-o-notch fa-spin",
			},
		},
		QUEUED: {
			CreatedAt: "2014-12-20 22:53:11",
			CommitID: "CQmmID123",
			ID: 1,
			expect: {
				cls: "primary",
				txt: "queued",
				icon: "fa-ellipsis-h",
			},
		},
	};

	Object.keys(renderTests).forEach((name) => {
		it(`should have correct classes, attributes and state when build is: ${name}`, () => {
			var test = renderTests[name];
			sandbox.stub(client, "builds", () => $.Deferred().resolve({Builds: [test]}).promise());

			var component = sandbox.renderComponent(
				<RepoBuildIndicator btnSize="test-size" RepoURI="test-uri" commitID="test-rev" />
			);
			var tag = TestUtils.findRenderedDOMComponentWithTag(component, "a");
			var $node = $(ReactDOM.findDOMNode(tag));

			expect(client.builds.callCount).to.be(1);
			expect(component.state.status).to.be(component.BuildStatus[name]);
			expect($node.hasClass(`text-${test.expect.cls}`)).to.be(true);
			expect($node.attr("href")).to.be(`/test-uri/.builds/${test.ID}`);
			expect($node.attr("title")).to.contain(`Build ${test.expect.txt}`);

			tag = TestUtils.findRenderedDOMComponentWithTag(component, "i");
			expect($(ReactDOM.findDOMNode(tag)).hasClass(test.expect.icon)).to.be(true);
		});
	}, this);

	it("should display build link when one is not available", () => {
		sandbox.stub(client, "builds", function() {
			return $.Deferred().resolve([]).promise();
		});

		var component = sandbox.renderComponent(
			<RepoBuildIndicator btnSize="test-size" RepoURI="test-uri" commitID="test-rev" Buildable="true" />
		);

		var tag = TestUtils.findRenderedDOMComponentWithTag(component, "a");
		var $node = $(ReactDOM.findDOMNode(tag));
		expect($node.attr("title")).to.contain("Build this version");
	});

	it("should not display build link when indicator is not buildable", () => {
		sandbox.stub(client, "builds", function() {
			return $.Deferred().resolve([]).promise();
		});

		var component = sandbox.renderComponent(
			<RepoBuildIndicator btnSize="test-size" RepoURI="test-uri" commitID="test-rev" />
		);

		var tag = TestUtils.findRenderedDOMComponentWithTag(component, "a");
		var $node = $(ReactDOM.findDOMNode(tag));
		expect($node.attr("title")).to.not.be.ok();
	});

	it("should request a new build and change cache when clicked with no build available", () => {
		sandbox.stub(client, "builds", () => $.Deferred().resolve([]).promise());
		sandbox.stub(client, "createRepoBuild", () => $.Deferred().resolve([]).promise());

		var component = sandbox.renderComponent(
			<RepoBuildIndicator btnSize="test-size" RepoURI="test-uri" commitID="test-rev" Buildable={true} />
		);
		expect(component.state.status).to.be(component.BuildStatus.NA);

		TestUtils.Simulate.click(TestUtils.findRenderedDOMComponentWithTag(component, "a"));
		expect(client.createRepoBuild.callCount).to.be(1);
		expect(component.state.noCache).to.be(true);
	});

	it("should render a span with class 'btn-danger' on error", () => {
		sandbox.stub(client, "builds", () => $.Deferred().reject().promise());

		var component = sandbox.renderComponent(
			<RepoBuildIndicator btnSize="test-size" RepoURI="test-uri" commitID="test-rev" />
		);
		expect(component.state.status).to.be(component.BuildStatus.ERROR);

		var tag = TestUtils.findRenderedDOMComponentWithTag(component, "a");
		var $node = $(ReactDOM.findDOMNode(tag));

		expect($node.hasClass("btn-danger")).to.be(true);
	});

	it("should not request build status if build is provided in props", () => {
		sandbox.stub(client, "builds");

		sandbox.renderComponent(
			<RepoBuildIndicator LastBuild={renderTests.FAILURE} btnSize="test-size" RepoURI="test-uri" commitID="test-rev" />
		);

		expect(client.builds.callCount).to.be(0);
	});

	it("should clear and start a new poller when build is STARTED", () => {
		sandbox.stub(global, "clearInterval");
		sandbox.stub(global, "setInterval");

		sandbox.renderComponent(
			<RepoBuildIndicator LastBuild={renderTests.STARTED} btnSize="test-size" RepoURI="test-uri" commitID="test-rev" />
		);

		expect(clearInterval.callCount).to.be(1);
		expect(setInterval.callCount).to.be(1);
	});

	it("should clear interval and not start a new poller when a build is BUILT", () => {
		sandbox.stub(global, "clearInterval");
		sandbox.stub(global, "setInterval");

		sandbox.renderComponent(
			<RepoBuildIndicator LastBuild={renderTests.BUILT} btnSize="test-size" RepoURI="test-uri" commitID="test-rev" />
		);

		expect(clearInterval.callCount).to.be(1);
		expect(setInterval.callCount).to.be(0);
	});

	it("should continue polling for build status QUEUED", () => {
		sandbox.stub(client, "builds", () => $.Deferred().resolve({Builds: [renderTests.QUEUED]}).promise());
		sandbox.useFakeTimers();
		sandbox.spy(global, "setInterval");

		sandbox.renderComponent(
			<RepoBuildIndicator btnSize="test-size" RepoURI="test-uri" commitID="test-rev" />
		);

		expect(client.builds.callCount).to.be(1);
		expect(setInterval.callCount).to.be(1);

		sandbox.clock.tick(10000);

		expect(client.builds.callCount).to.be(2);
		expect(setInterval.callCount).to.be(2);
	});

	it("should poll for build status if props is STARTED, and stop after FAILURE", () => {
		sandbox.stub(client, "builds", () => $.Deferred().resolve({Builds: [renderTests.FAILURE]}));
		sandbox.useFakeTimers();
		sandbox.spy(global, "clearInterval");
		sandbox.spy(global, "setInterval");

		sandbox.renderComponent(
			<RepoBuildIndicator LastBuild={renderTests.STARTED} btnSize="test-size" RepoURI="test-uri" commitID="test-rev" />
		);

		expect(client.builds.callCount).to.be(0);
		expect(clearInterval.callCount).to.be(1);
		expect(setInterval.callCount).to.be(1);

		sandbox.clock.tick(5000);

		expect(client.builds.callCount).to.be(1);
		expect(clearInterval.callCount).to.be(2);
		expect(setInterval.callCount).to.be(1);
	});

	it("should reload page when succeeding with success-reload on", () => {
		global.location = {reload: sandbox.stub()};
		sandbox.stub(client, "builds", () => $.Deferred().resolve({Builds: [renderTests.BUILT]}));
		sandbox.useFakeTimers();

		sandbox.renderComponent(
			<RepoBuildIndicator LastBuild={renderTests.STARTED} btnSize="test-size" RepoURI="test-uri" commitID="test-rev" SuccessReload="on" />
		);

		sandbox.clock.tick(5000);

		expect(location.reload.callCount).to.be(1);
	});

	it("should not reload page when succeeding with success-reload undefined", () => {
		global.location = {reload: sandbox.stub()};
		sandbox.stub(client, "builds", () => $.Deferred().resolve({Builds: [renderTests.BUILT]}));
		sandbox.useFakeTimers();

		sandbox.renderComponent(
			<RepoBuildIndicator LastBuild={renderTests.STARTED} btnSize="test-size" RepoURI="test-uri" commitID="test-rev" />
		);

		sandbox.clock.tick(5000);

		expect(location.reload.callCount).to.be(0);
	});
});
