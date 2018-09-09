import { Notifications } from '@sourcegraph/extensions-client-common/lib/app/notifications/Notifications'
import { createController as createExtensionsController } from '@sourcegraph/extensions-client-common/lib/client/controller'
import { ConfiguredExtension } from '@sourcegraph/extensions-client-common/lib/extensions/extension'
import {
    ConfigurationCascadeOrError,
    ConfigurationSubject,
    ConfiguredSubject,
    Settings,
} from '@sourcegraph/extensions-client-common/lib/settings'
import ErrorIcon from '@sourcegraph/icons/lib/Error'
import ServerIcon from '@sourcegraph/icons/lib/Server'
import * as React from 'react'
import { Route } from 'react-router'
import { BrowserRouter } from 'react-router-dom'
import { Subscription } from 'rxjs'
import {
    Component as ExtensionsComponent,
    EMPTY_ENVIRONMENT as EXTENSIONS_EMPTY_ENVIRONMENT,
} from 'sourcegraph/module/environment/environment'
import { URI } from 'sourcegraph/module/types/textDocument'
import { currentUser } from './auth'
import * as GQL from './backend/graphqlschema'
import { FeedbackText } from './components/FeedbackText'
import { HeroPage } from './components/HeroPage'
import { Tooltip } from './components/tooltip/Tooltip'
import { ExtensionsEnvironmentProps, USE_PLATFORM } from './extensions/environment/ExtensionsEnvironment'
import {
    ConfigurationCascadeProps,
    createMessageTransports,
    ExtensionsControllerProps,
    ExtensionsProps,
} from './extensions/ExtensionsClientCommonContext'
import { createExtensionsContextController } from './extensions/ExtensionsClientCommonContext'
import { Layout, LayoutProps } from './Layout'
import { updateUserSessionStores } from './marketing/util'
import { SiteAdminAreaRoute } from './site-admin/SiteAdminArea'
import { SiteAdminSideBarItems } from './site-admin/SiteAdminSidebar'
import { eventLogger } from './tracking/eventLogger'
import { UserAccountAreaRoute } from './user/account/UserAccountArea'
import { UserAccountSidebarItems } from './user/account/UserAccountSidebar'
import { isErrorLike } from './util/errors'

export interface SourcegraphWebAppProps {
    siteAdminAreaRoutes: ReadonlyArray<SiteAdminAreaRoute>
    siteAdminSideBarItems: SiteAdminSideBarItems
    userAccountSideBarItems: UserAccountSidebarItems
    userAccountAreaRoutes: ReadonlyArray<UserAccountAreaRoute>
}

interface SourcegraphWebAppState
    extends ConfigurationCascadeProps,
        ExtensionsProps,
        ExtensionsEnvironmentProps,
        ExtensionsControllerProps {
    error?: Error
    user?: GQL.IUser | null

    /**
     * Whether the light theme is enabled or not
     */
    isLightTheme: boolean

    /**
     * The current search query in the navbar.
     */
    navbarSearchQuery: string

    /** Whether the help popover is shown. */
    showHelpPopover: boolean

    /** Whether the history popover is shown. */
    showHistoryPopover: boolean
}

const LIGHT_THEME_LOCAL_STORAGE_KEY = 'light-theme'

/** A fallback configuration subject that can be constructed synchronously at initialization time. */
const SITE_SUBJECT_NO_ADMIN: Pick<GQL.IConfigurationSubject, 'id' | 'viewerCanAdminister'> = {
    id: window.context.siteGQLID,
    viewerCanAdminister: false,
}

/**
 * The root component
 */
export class SourcegraphWebApp extends React.Component<SourcegraphWebAppProps, SourcegraphWebAppState> {
    constructor(props: SourcegraphWebAppProps) {
        super(props)
        const extensions = createExtensionsContextController()
        this.state = {
            isLightTheme: localStorage.getItem(LIGHT_THEME_LOCAL_STORAGE_KEY) !== 'false',
            navbarSearchQuery: '',
            showHelpPopover: false,
            showHistoryPopover: false,
            configurationCascade: { subjects: null, merged: null },
            extensions,
            extensionsEnvironment: EXTENSIONS_EMPTY_ENVIRONMENT,
            extensionsController: createExtensionsController(extensions.context, createMessageTransports),
        }
    }

    private subscriptions = new Subscription()

    public componentDidMount(): void {
        updateUserSessionStores()

        document.body.classList.add('theme')
        this.subscriptions.add(
            currentUser.subscribe(user => this.setState({ user }), () => this.setState({ user: null }))
        )

        if (USE_PLATFORM) {
            this.subscriptions.add(this.state.extensionsController)

            this.subscriptions.add(
                this.state.extensions.context.configurationCascade.subscribe(
                    v => this.onConfigurationCascadeChange(v),
                    err => console.error(err)
                )
            )

            // Keep the Sourcegraph extensions controller's extensions up-to-date.
            //
            // TODO(sqs): handle loading and errors
            this.subscriptions.add(
                this.state.extensions.viewerConfiguredExtensions.subscribe(
                    extensions => this.onViewerConfiguredExtensionsChange(extensions),
                    err => console.error(err)
                )
            )
        }
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
        document.body.classList.remove('theme')
        document.body.classList.remove('theme-light')
        document.body.classList.remove('theme-dark')
    }

    public componentDidUpdate(): void {
        localStorage.setItem(LIGHT_THEME_LOCAL_STORAGE_KEY, this.state.isLightTheme + '')
        document.body.classList.toggle('theme-light', this.state.isLightTheme)
        document.body.classList.toggle('theme-dark', !this.state.isLightTheme)
    }

    public render(): React.ReactFragment | null {
        if (this.state.error) {
            return <HeroPage icon={ErrorIcon} title={'Something happened'} subtitle={this.state.error.message} />
        }

        if (window.pageError && window.pageError.statusCode !== 404) {
            const statusCode = window.pageError.statusCode
            const statusText = window.pageError.statusText
            const errorMessage = window.pageError.error
            const errorID = window.pageError.errorID

            let subtitle: JSX.Element | undefined
            if (errorID) {
                subtitle = <FeedbackText headerText="Sorry, there's been a problem." />
            }
            if (errorMessage) {
                subtitle = (
                    <div className="app__error">
                        {subtitle}
                        {subtitle && <hr />}
                        <pre>{errorMessage}</pre>
                    </div>
                )
            } else {
                subtitle = <div className="app__error">{subtitle}</div>
            }
            return <HeroPage icon={ServerIcon} title={`${statusCode}: ${statusText}`} subtitle={subtitle} />
        }

        const { user } = this.state
        if (user === undefined) {
            return null
        }

        return (
            <>
                <BrowserRouter key={0}>
                    <Route
                        path="/"
                        // tslint:disable-next-line:jsx-no-lambda RouteProps.render is an exception
                        render={routeComponentProps => {
                            let viewerSubject: LayoutProps['viewerSubject']
                            if (this.state.user) {
                                viewerSubject = this.state.user
                            } else if (
                                this.state.configurationCascade &&
                                !isErrorLike(this.state.configurationCascade) &&
                                this.state.configurationCascade.subjects &&
                                !isErrorLike(this.state.configurationCascade.subjects) &&
                                this.state.configurationCascade.subjects.length > 0
                            ) {
                                viewerSubject = this.state.configurationCascade.subjects[0].subject
                            } else {
                                viewerSubject = SITE_SUBJECT_NO_ADMIN
                            }

                            return (
                                <Layout
                                    {...routeComponentProps}
                                    user={user}
                                    siteAdminAreaRoutes={this.props.siteAdminAreaRoutes}
                                    siteAdminSideBarItems={this.props.siteAdminSideBarItems}
                                    userAccountSideBarItems={this.props.userAccountSideBarItems}
                                    userAccountAreaRoutes={this.props.userAccountAreaRoutes}
                                    viewerSubject={viewerSubject}
                                    isLightTheme={this.state.isLightTheme}
                                    onThemeChange={this.onThemeChange}
                                    navbarSearchQuery={this.state.navbarSearchQuery}
                                    onNavbarQueryChange={this.onNavbarQueryChange}
                                    showHelpPopover={this.state.showHelpPopover}
                                    showHistoryPopover={this.state.showHistoryPopover}
                                    onHelpPopoverToggle={this.onHelpPopoverToggle}
                                    onHistoryPopoverToggle={this.onHistoryPopoverToggle}
                                    configurationCascade={this.state.configurationCascade}
                                    extensions={this.state.extensions}
                                    extensionsEnvironment={this.state.extensionsEnvironment}
                                    extensionsOnComponentChange={this.extensionsOnComponentChange}
                                    extensionsOnRootChange={this.extensionsOnRootChange}
                                    extensionsController={this.state.extensionsController}
                                />
                            )
                        }}
                    />
                </BrowserRouter>
                <Tooltip key={1} />
                {USE_PLATFORM ? <Notifications key={2} extensionsController={this.state.extensionsController} /> : null}
            </>
        )
    }

    private onThemeChange = () => {
        this.setState(
            state => ({ isLightTheme: !state.isLightTheme }),
            () => {
                eventLogger.log(this.state.isLightTheme ? 'LightThemeClicked' : 'DarkThemeClicked')
            }
        )
    }

    private onNavbarQueryChange = (navbarSearchQuery: string) => {
        this.setState({ navbarSearchQuery })
    }

    private onHelpPopoverToggle = (visible?: boolean): void => {
        eventLogger.log('HelpPopoverToggled')
        this.setState(prevState => ({
            // If visible is any non-boolean type (e.g., MouseEvent), treat it as undefined. This lets callers use
            // onHelpPopoverToggle directly in an event handler without wrapping it in an another function.
            showHelpPopover: visible !== true && visible !== false ? !prevState.showHelpPopover : visible,
        }))
    }

    private onHistoryPopoverToggle = (visible?: boolean): void => {
        eventLogger.log('HistoryPopoverToggled')
        this.setState(prevState => ({
            showHistoryPopover: visible !== true && visible !== false ? !prevState.showHistoryPopover : visible,
        }))
    }

    private onConfigurationCascadeChange(
        configurationCascade: ConfigurationCascadeOrError<ConfigurationSubject, Settings>
    ): void {
        this.setState(
            prevState => {
                const update: Pick<SourcegraphWebAppState, 'configurationCascade' | 'extensionsEnvironment'> = {
                    configurationCascade,
                    extensionsEnvironment: prevState.extensionsEnvironment,
                }
                if (
                    configurationCascade.subjects !== null &&
                    !isErrorLike(configurationCascade.subjects) &&
                    configurationCascade.merged !== null &&
                    !isErrorLike(configurationCascade.merged)
                ) {
                    // Only update Sourcegraph extensions environment configuration if the configuration was
                    // successfully parsed.
                    //
                    // TODO(sqs): Think through how this error should be handled.
                    update.extensionsEnvironment = {
                        ...prevState.extensionsEnvironment,
                        configuration: {
                            subjects: configurationCascade.subjects.filter(
                                (subject): subject is ConfiguredSubject<ConfigurationSubject, Settings> =>
                                    subject.settings !== null && !isErrorLike(subject.settings)
                            ),
                            merged: configurationCascade.merged,
                        },
                    }
                }
                return update
            },
            () => this.state.extensionsController.setEnvironment(this.state.extensionsEnvironment)
        )
    }

    private onViewerConfiguredExtensionsChange(viewerConfiguredExtensions: ConfiguredExtension[]): void {
        this.setState(
            prevState => ({
                extensionsEnvironment: {
                    ...prevState.extensionsEnvironment,
                    extensions: viewerConfiguredExtensions,
                },
            }),
            () => this.state.extensionsController.setEnvironment(this.state.extensionsEnvironment)
        )
    }

    private extensionsOnComponentChange = (component: ExtensionsComponent | null): void => {
        this.setState(
            prevState => ({ extensionsEnvironment: { ...prevState.extensionsEnvironment, component } }),
            () => this.state.extensionsController.setEnvironment(this.state.extensionsEnvironment)
        )
    }

    private extensionsOnRootChange = (root: URI | null): void => {
        this.setState(
            prevState => ({ extensionsEnvironment: { ...prevState.extensionsEnvironment, root } }),
            () => this.state.extensionsController.setEnvironment(this.state.extensionsEnvironment)
        )
    }
}
